package languages

import (
	"path"
	"regexp"
	"strings"

	"github.com/felukka/koptan/internal/service/repofs"
)

// Python installs dependencies into a virtualenv and runs the app with the
// server its framework expects.
type Python struct{}

const pythonDefaultVersion = "3.12"

var (
	pyFastAPIRe = regexp.MustCompile(`(?m)^(\w+)\s*=\s*FastAPI\(`)
	pyFlaskRe   = regexp.MustCompile(`(?m)^(\w+)\s*=\s*Flask\(`)
	// pyAppFiles are where an ASGI/WSGI app object usually lives.
	pyAppFiles = []string{"main.py", "app.py", "server.py", "app/main.py", "src/main.py", "api/main.py"}
)

func (Python) Name() string { return "python" }

func (Python) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("python", pythonDefaultVersion)
	pyproject := fs.ReadString("pyproject.toml")
	switch {
	case fs.Exists("uv.lock"):
		f.PackageManager = "uv"
		f.InstallCmd = "pip install --no-cache-dir uv && uv sync --frozen --no-dev --no-install-project"
	case fs.Exists("poetry.lock") || repofs.TOMLHasSection(pyproject, "tool.poetry"):
		f.PackageManager = "poetry"
		f.InstallCmd = "pip install --no-cache-dir poetry && poetry config virtualenvs.create false && " +
			"poetry install --only main --no-root --no-interaction"
	case fs.Exists("Pipfile"):
		f.PackageManager = "pipenv"
		f.InstallCmd = "pip install --no-cache-dir pipenv && pipenv install --deploy --system"
	case fs.Exists("requirements.txt"):
		f.PackageManager = "pip"
		f.InstallCmd = "pip install --no-cache-dir -r requirements.txt"
	case pyproject != "" || fs.Exists("setup.py"):
		f.PackageManager = "pip"
		f.InstallCmd = "pip install --no-cache-dir ."
	default:
		return f, false
	}
	f.Version = pythonVersion(fs, pyproject)
	pythonEntrypoint(fs, f, pyproject)
	return f, true
}

func pythonVersion(fs *repofs.FS, pyproject string) string {
	if v := firstLine(fs.ReadString(".python-version")); v != "" {
		return majorMinor(v, pythonDefaultVersion)
	}
	if v := repofs.TOMLValue(pyproject, "project", "requires-python"); v != "" {
		return majorMinor(v, pythonDefaultVersion)
	}
	return pythonDefaultVersion
}

// pythonEntrypoint picks the start command from the framework in use.
func pythonEntrypoint(fs *repofs.FS, f *Facts, pyproject string) {
	deps := strings.ToLower(pyproject + fs.ReadString("requirements.txt") + fs.ReadString("Pipfile"))
	switch {
	case fs.Exists("manage.py") || strings.Contains(deps, "django"):
		f.Framework = "django"
		f.Entrypoint = djangoWSGI(fs)
		f.Extra["extraPackages"] = "gunicorn"
		f.StartCmd = shellCmd("gunicorn " + f.Entrypoint + " --bind 0.0.0.0:${PORT}")
		return
	case strings.Contains(deps, "fastapi"):
		if module, app := findPyApp(fs, pyFastAPIRe); module != "" {
			f.Framework = "fastapi"
			f.Entrypoint = module + ":" + app
			f.Extra["extraPackages"] = "uvicorn"
			f.StartCmd = shellCmd("uvicorn " + f.Entrypoint + " --host 0.0.0.0 --port ${PORT}")
			return
		}
	case strings.Contains(deps, "flask"):
		if module, app := findPyApp(fs, pyFlaskRe); module != "" {
			f.Framework = "flask"
			f.Entrypoint = module + ":" + app
			f.Extra["extraPackages"] = "gunicorn"
			f.StartCmd = shellCmd("gunicorn " + f.Entrypoint + " --bind 0.0.0.0:${PORT}")
			return
		}
	}
	f.Entrypoint = fs.FirstExisting("main.py", "app.py", "server.py", "src/main.py")
	if f.Entrypoint == "" {
		f.Entrypoint = "main.py"
	}
	f.StartCmd = []string{"python", f.Entrypoint}
}

// findPyApp returns the module (dotted) and variable of an app object.
func findPyApp(fs *repofs.FS, re *regexp.Regexp) (module, app string) {
	for _, file := range pyAppFiles {
		if m := re.FindStringSubmatch(fs.ReadString(file)); m != nil {
			return strings.ReplaceAll(strings.TrimSuffix(file, ".py"), "/", "."), m[1]
		}
	}
	return "", ""
}

// djangoWSGI finds <project>/wsgi.py next to manage.py.
func djangoWSGI(fs *repofs.FS) string {
	for _, dir := range fs.List(".", true) {
		if fs.Exists(path.Join(dir, "wsgi.py")) {
			return dir + ".wsgi:application"
		}
	}
	return "wsgi:application"
}
