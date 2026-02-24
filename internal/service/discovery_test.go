package service

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/felukka/koptan/internal/service/languages"
	"github.com/felukka/koptan/internal/service/repofs"
)

// Run `go test ./internal/service -update` to rewrite the golden files.
var update = flag.Bool("update", false, "rewrite testdata/golden")

func discoverFixture(t *testing.T, fixture string, req Request) (*Result, error) {
	t.Helper()
	root, err := repofs.New(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	if req.Port == 0 {
		req.Port = 8080
	}
	return NewEngine().DiscoverDir(root, req)
}

func TestDiscoverGeneratesDockerfiles(t *testing.T) {
	cases := []struct {
		fixture string
		req     Request
		want    languages.Facts
	}{
		{"go-cmd", Request{}, languages.Facts{Language: "go", Version: "1.23", Entrypoint: "./cmd/shop"}},
		{"go-root", Request{}, languages.Facts{Language: "go", Version: "1.22", Entrypoint: "."}},
		{"node-pnpm", Request{}, languages.Facts{Language: "node", Version: "20", PackageManager: "pnpm",
			Entrypoint: "start script"}},
		{"node-npm-main", Request{Port: 3000}, languages.Facts{Language: "node", Version: "18",
			PackageManager: "npm", Entrypoint: "server.js"}},
		{"python-fastapi-poetry", Request{}, languages.Facts{Language: "python", Version: "3.11",
			PackageManager: "poetry", Framework: "fastapi", Entrypoint: "app.main:api"}},
		{"python-django", Request{}, languages.Facts{Language: "python", Version: "3.12",
			PackageManager: "pip", Framework: "django", Entrypoint: "mysite.wsgi:application"}},
		{"python-flask", Request{}, languages.Facts{Language: "python", Version: "3.12",
			PackageManager: "pip", Framework: "flask", Entrypoint: "app:application"}},
		{"java-maven", Request{}, languages.Facts{Language: "java", Version: "17", PackageManager: "maven",
			Entrypoint: "app.jar"}},
		{"java-gradle-wrapper", Request{}, languages.Facts{Language: "java", Version: "25",
			PackageManager: "gradle", Entrypoint: "app.jar"}},
		{"dotnet", Request{}, languages.Facts{Language: "dotnet", Version: "9.0", PackageManager: "nuget",
			Entrypoint: "Shop.Api"}},
		{"rust", Request{}, languages.Facts{Language: "rust", Version: "1.80", PackageManager: "cargo",
			Entrypoint: "hello"}},
		{"ruby-rails", Request{}, languages.Facts{Language: "ruby", Version: "3.2", PackageManager: "bundler",
			Framework: "rails", Entrypoint: "config.ru"}},
		{"php-laravel", Request{}, languages.Facts{Language: "php", Version: "8.2", PackageManager: "composer",
			Framework: "laravel", Entrypoint: "index.php"}},
		{"static", Request{}, languages.Facts{Language: "static", Version: "1.27", PackageManager: "none",
			Entrypoint: "public/index.html"}},
		{"monorepo", Request{ContextDir: "services/api"}, languages.Facts{Language: "node", Version: "22",
			PackageManager: "yarn", Entrypoint: "start script"}},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			res, err := discoverFixture(t, tc.fixture, tc.req)
			if err != nil {
				t.Fatalf("discover: %v", err)
			}
			got := res.Facts
			for field, pair := range map[string][2]string{
				"language":       {got.Language, tc.want.Language},
				"version":        {got.Version, tc.want.Version},
				"packageManager": {got.PackageManager, tc.want.PackageManager},
				"framework":      {got.Framework, tc.want.Framework},
				"entrypoint":     {got.Entrypoint, tc.want.Entrypoint},
			} {
				if pair[1] != "" && pair[0] != pair[1] {
					t.Errorf("%s = %q, want %q", field, pair[0], pair[1])
				}
			}
			if res.Source != SourceTemplate {
				t.Errorf("source = %q, want %q", res.Source, SourceTemplate)
			}
			checkGenerated(t, res.Dockerfile, tc.req.Port)
			compareGolden(t, tc.fixture, res.Dockerfile)
		})
	}
}

// checkGenerated holds every generated Dockerfile to the same rules: it
// has a FROM, exports and exposes $PORT, and its final user is not root.
func checkGenerated(t *testing.T, dockerfile []byte, port int32) {
	t.Helper()
	if port == 0 {
		port = 8080
	}
	text := string(dockerfile)
	if err := ValidateDockerfile(dockerfile); err != nil {
		t.Error(err)
	}
	p := strconv.Itoa(int(port))
	for _, want := range []string{"PORT=" + p, "EXPOSE " + p} {
		if !strings.Contains(text, want) {
			t.Errorf("Dockerfile lacks %q:\n%s", want, text)
		}
	}
	lastUser := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "USER ") {
			lastUser = strings.TrimPrefix(line, "USER ")
		}
	}
	if lastUser == "" || lastUser == "root" || strings.HasPrefix(lastUser, "0") {
		t.Errorf("final USER is %q, want a non-root user:\n%s", lastUser, text)
	}
}

func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".Dockerfile")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Dockerfile differs from %s (run with -update after checking):\n%s", path, got)
	}
}

func TestDiscoverPrefersRepositoryDockerfile(t *testing.T) {
	res, err := discoverFixture(t, "repo-dockerfile", Request{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceRepo || !strings.HasPrefix(string(res.Dockerfile), "FROM scratch") {
		t.Errorf("want the repository Containerfile, got source %q:\n%s", res.Source, res.Dockerfile)
	}
	if res.Language() != "go" || res.Dockerignore != nil {
		t.Errorf("language = %q, dockerignore = %q", res.Language(), res.Dockerignore)
	}
}

func TestDiscoverDockerignore(t *testing.T) {
	res, err := discoverFixture(t, "go-root", Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Dockerignore), ".git") {
		t.Errorf("want a default .dockerignore, got %q", res.Dockerignore)
	}
	res, err = discoverFixture(t, "monorepo", Request{ContextDir: "services/api"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dockerignore != nil {
		t.Error("a repository .dockerignore must not be replaced")
	}
}

func TestDiscoverOverridesAndErrors(t *testing.T) {
	res, err := discoverFixture(t, "repo-dockerfile", Request{Language: "static"})
	if err != nil || res.Language() != "static" || res.Source != SourceRepo {
		t.Errorf("language override: %v, %+v", err, res)
	}
	errorCases := []struct {
		fixture, want string
		req           Request
	}{
		{"unknown", "no Dockerfile and no supported stack", Request{}},
		{"go-root", "is not supported", Request{Language: "cobol"}},
		{"go-root", "dockerfilePath", Request{DockerfilePath: "missing/Dockerfile"}},
		{"go-root", "contextDir", Request{ContextDir: "nope"}},
		{"evil-name", "spans several lines", Request{}},
	}
	for _, tc := range errorCases {
		_, err := discoverFixture(t, tc.fixture, tc.req)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %+v: error %v, want %q", tc.fixture, tc.req, err, tc.want)
		}
	}
}

func TestEveryLanguageHasATemplate(t *testing.T) {
	for _, l := range languages.Registry() {
		facts, _ := l.Detect(mustFS(t, t.TempDir()))
		facts.Port = 8080
		if _, err := languages.Render(facts); err != nil {
			t.Errorf("%s: %v", l.Name(), err)
		}
	}
}

func mustFS(t *testing.T, dir string) *repofs.FS {
	t.Helper()
	fs, err := repofs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}
