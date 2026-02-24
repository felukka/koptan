package languages

import (
	"path"
	"regexp"
	"strings"

	"github.com/felukka/koptan/internal/service/repofs"
)

// Go builds a static binary and runs it on distroless.
type Go struct{}

const goDefaultVersion = "1.24"

var (
	goDirectiveRe = regexp.MustCompile(`(?m)^go\s+(\S+)`)
	goModuleRe    = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	goMainRe      = regexp.MustCompile(`(?m)^package\s+main\b`)
)

func (Go) Name() string { return "go" }

func (Go) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("go", goDefaultVersion)
	f.PackageManager = "go modules"
	f.Entrypoint = "."
	mod := fs.ReadString("go.mod")
	if mod == "" {
		return f, false
	}
	if m := goDirectiveRe.FindStringSubmatch(mod); m != nil {
		f.Version = majorMinor(m[1], goDefaultVersion)
	}
	module := ""
	if m := goModuleRe.FindStringSubmatch(mod); m != nil {
		module = path.Base(m[1])
	}
	f.Entrypoint = goMainPackage(fs, module)
	f.BuildCmd = `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ` + f.Entrypoint
	f.StartCmd = []string{"/app"}
	return f, true
}

// goMainPackage finds the package to build: the root when it is a main
// package, else one under cmd/ (preferring cmd/<module>, cmd/server,
// cmd/api, then the only or first one).
func goMainPackage(fs *repofs.FS, module string) string {
	if isGoMain(fs, ".") {
		return "."
	}
	var mains []string
	for _, dir := range fs.List("cmd", true) {
		if isGoMain(fs, path.Join("cmd", dir)) {
			mains = append(mains, dir)
		}
	}
	if len(mains) == 0 {
		return "."
	}
	for _, preferred := range []string{module, "server", "api", "app", "main"} {
		for _, m := range mains {
			if m == preferred {
				return "./cmd/" + m
			}
		}
	}
	return "./cmd/" + mains[0]
}

// isGoMain reports whether dir holds a non-test file of package main.
func isGoMain(fs *repofs.FS, dir string) bool {
	for _, file := range fs.Glob(dir, "*.go") {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		if goMainRe.MatchString(fs.ReadString(file)) {
			return true
		}
	}
	return false
}
