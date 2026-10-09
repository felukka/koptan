package languages

import (
	"github.com/felukka/koptan/internal/service/repofs"
)

// Node installs with the repository's package manager, runs its build
// script, and starts it with its start script or main file.
type Node struct{}

const (
	nodeDefaultVersion = "22"
	bun                = "bun"
)

type packageJSON struct {
	Main    string            `json:"main"`
	Scripts map[string]string `json:"scripts"`
	Engines map[string]string `json:"engines"`
}

// nodePM describes how one package manager installs, builds and starts.
type nodePM struct {
	name, lockfile, install, prune string
	corepack                       bool
}

var nodePMs = []nodePM{
	{name: "pnpm", lockfile: "pnpm-lock.yaml", install: "pnpm install --frozen-lockfile",
		prune: "pnpm prune --prod", corepack: true},
	{name: "yarn", lockfile: "yarn.lock", install: "yarn install --frozen-lockfile", corepack: true},
	{name: bun, lockfile: "bun.lock", install: "bun install --frozen-lockfile"},
	{name: bun, lockfile: "bun.lockb", install: "bun install --frozen-lockfile"},
	{name: "npm", lockfile: "package-lock.json", install: "npm ci", prune: "npm prune --omit=dev"},
	{name: "npm", lockfile: "npm-shrinkwrap.json", install: "npm ci", prune: "npm prune --omit=dev"},
}

func (Node) Name() string { return "node" }

func (Node) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("node", nodeDefaultVersion)
	var pkg packageJSON
	if err := fs.ReadJSON("package.json", &pkg); err != nil {
		f.PackageManager = "npm"
		f.InstallCmd = "npm install"
		f.StartCmd = []string{"npm", "start"}
		return f, false
	}
	pm := nodePM{name: "npm", install: "npm install", prune: "npm prune --omit=dev"}
	for _, candidate := range nodePMs {
		if fs.Exists(candidate.lockfile) {
			pm = candidate
			break
		}
	}
	if pm.name == "yarn" && fs.Exists(".yarnrc.yml") {
		pm.install = "yarn install --immutable" // Yarn 2+
	}
	f.PackageManager = pm.name
	f.InstallCmd = pm.install
	f.Extra["prune"] = pm.prune
	if pm.corepack {
		f.Extra["corepack"] = "true"
	}
	if pm.name == bun {
		f.Extra["image"] = "oven/bun:1"
	}

	f.Version = nodeVersion(fs, pkg)
	if _, ok := pkg.Scripts["build"]; ok {
		f.BuildCmd = pm.name + " run build"
	}
	switch {
	case pkg.Scripts["start"] != "":
		// npm runs any package manager's scripts and is always in the image.
		runner := "npm"
		if pm.name == bun {
			runner = bun
		}
		f.Entrypoint = "start script"
		f.StartCmd = []string{runner, "run", "start"}
	default:
		f.Entrypoint = pkg.Main
		if f.Entrypoint == "" {
			f.Entrypoint = fs.FirstExisting("server.js", "index.js", "app.js", "main.js", "dist/index.js")
		}
		if f.Entrypoint == "" {
			f.Entrypoint = "index.js"
		}
		runtime := "node"
		if pm.name == bun {
			runtime = bun
		}
		f.StartCmd = []string{runtime, f.Entrypoint}
	}
	return f, true
}

// nodeVersion reads .nvmrc, .node-version or engines.node; "lts" or
// anything without a number keeps the default.
func nodeVersion(fs *repofs.FS, pkg packageJSON) string {
	for _, file := range []string{".nvmrc", ".node-version"} {
		if v := firstLine(fs.ReadString(file)); v != "" {
			return major(v, nodeDefaultVersion)
		}
	}
	return major(pkg.Engines["node"], nodeDefaultVersion)
}
