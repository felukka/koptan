package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// Stubs for the tools codeQLScript calls. Each logs its arguments to $CALLS.
const (
	stubCurl = `#!/bin/sh
echo "curl $*" >> "$CALLS"
while [ $# -gt 0 ]; do [ "$1" = -o ] && { printf bundle > "$2"; shift; }; shift; done
`
	stubTar = `#!/bin/sh
echo "tar $*" >> "$CALLS"
mkdir -p "$CODEQL_WORK_DIR/codeql" && cp "$STUB_CODEQL" "$CODEQL_WORK_DIR/codeql/codeql"
`
	// The codeql stub writes $STUB_SARIF on analyze and fails every upload,
	// like a repository without code scanning (HTTP 403).
	stubCodeQL = `#!/bin/sh
echo "codeql $*" >> "$CALLS"
case "$1 $2" in
  "database analyze") for a in "$@"; do case "$a" in --output=*) cp "$STUB_SARIF" "${a#--output=}" ;; esac; done ;;
  "github upload-results") echo "HTTP 403: Code scanning is not enabled" >&2; exit 1 ;;
esac
`
)

type scriptRun struct {
	out   string
	calls string
	err   error
}

// runCodeQLScript runs codeQLScript against the stubs, with the real jq and
// sha256sum. env overrides the defaults below.
func runCodeQLScript(t *testing.T, sarif string, env map[string]string) scriptRun {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("no jq")
	}
	if _, err := exec.LookPath("sha256sum"); err != nil {
		t.Skip("no sha256sum")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	for name, body := range map[string]string{"curl": stubCurl, "tar": stubTar, "codeql-stub": stubCodeQL} {
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(jq, filepath.Join(bin, "jq")); err != nil {
		t.Fatal(err)
	}
	sarifPath, err := filepath.Abs(sarif)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("bundle"))
	vars := map[string]string{
		"PATH":                 bin + ":" + os.Getenv("PATH"),
		"CALLS":                filepath.Join(dir, "calls"),
		"STUB_CODEQL":          filepath.Join(bin, "codeql-stub"),
		"STUB_SARIF":           sarifPath,
		"CODEQL_WORK_DIR":      dir,
		"CODEQL_LANGUAGES":     "python:python",
		"CODEQL_SUITE":         "code-scanning",
		"CODEQL_FAIL_ON":       "error",
		"CODEQL_BUNDLE_URL":    "https://example.com/codeql-bundle.tar.gz",
		"CODEQL_BUNDLE_SHA256": hex.EncodeToString(sum[:]),
		"KOPTAN_LANGUAGE":      "python",
		"KOPTAN_REPORTS_DIR":   filepath.Join(dir, "reports"),
		"KOPTAN_SOURCE_DIR":    dir,
		"KOPTAN_PLUGIN":        "codeql",
		"KOPTAN_REVISION":      "0123456789abcdef0123456789abcdef01234567",
		"GITHUB_TOKEN":         "t",
		"GITHUB_REPOSITORY":    "org/repo",
		"GITHUB_REF":           "refs/heads/main",
	}
	for k, v := range env {
		vars[k] = v
	}
	cmd := exec.Command(sh, "-c", codeQLScript)
	for k, v := range vars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	calls, _ := os.ReadFile(vars["CALLS"])
	return scriptRun{out: string(out), calls: string(calls), err: err}
}

// P-24 + P-22: real CodeQL SARIF with 3 error-level rules fails the gate,
// and the failing upload neither hides nor decides the verdict.
func TestCodeQLScriptFailsOnFindingsDespiteUploadError(t *testing.T) {
	r := runCodeQLScript(t, "testdata/codeql-python.sarif", nil)
	if r.err == nil {
		t.Fatalf("a repo with error findings must fail the step:\n%s", r.out)
	}
	if !strings.Contains(r.out, "python: 3 result(s) at or above error") {
		t.Errorf("verdict missing:\n%s", r.out)
	}
	if !strings.Contains(r.out, "warning: SARIF upload") {
		t.Errorf("upload failure must be reported as a warning:\n%s", r.out)
	}
}

func TestCodeQLScriptPassesCleanRepoDespiteUploadError(t *testing.T) {
	r := runCodeQLScript(t, "testdata/codeql-clean.sarif", nil)
	if r.err != nil {
		t.Fatalf("a clean repo must pass even when the upload fails: %v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.calls, "codeql github upload-results") {
		t.Errorf("the upload must still be attempted:\n%s", r.calls)
	}
}

// P-10: no language must fail closed, not skip.
func TestCodeQLScriptFailsWithoutLanguage(t *testing.T) {
	r := runCodeQLScript(t, "testdata/codeql-clean.sarif", map[string]string{"CODEQL_LANGUAGES": "", "KOPTAN_LANGUAGE": ""})
	if r.err == nil || !strings.Contains(r.out, "no CodeQL language") {
		t.Fatalf("must fail with a clear message, got err=%v:\n%s", r.err, r.out)
	}
	if r.calls != "" {
		t.Errorf("nothing may be downloaded or run:\n%s", r.calls)
	}
}

// P-13: a bundle that doesn't match the pinned digest is never unpacked.
func TestCodeQLScriptRejectsBadBundleDigest(t *testing.T) {
	r := runCodeQLScript(t, "testdata/codeql-clean.sarif", map[string]string{"CODEQL_BUNDLE_SHA256": strings.Repeat("0", 64)})
	if r.err == nil {
		t.Fatalf("a digest mismatch must fail:\n%s", r.out)
	}
	if strings.Contains(r.calls, "tar ") || strings.Contains(r.calls, "codeql ") {
		t.Errorf("the bundle must not be unpacked or run:\n%s", r.calls)
	}
	if !strings.Contains(r.calls, "--retry 3") {
		t.Errorf("the download must retry:\n%s", r.calls)
	}
}

func TestCodeQLDefaultsArePinned(t *testing.T) {
	p := plugin("codeql", 1, koptanv1.CIPluginSpec{Type: koptanv1.CIPluginCodeQL, CodeQL: &koptanv1.CodeQLConfig{}})
	c, err := Step(&p, BuildEnv{Language: "python"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Image != "docker.io/library/python:3.12-slim-bookworm" {
		t.Errorf("default image must have python3 and a registry host, got %q", c.Image)
	}
	if strings.Contains(codeQLBundleURL, "/latest/") || !hasEnv(c.Env, "CODEQL_BUNDLE_SHA256", codeQLBundleSHA256) {
		t.Errorf("default bundle must be pinned and checked: %s %+v", codeQLBundleURL, c.Env)
	}
	custom := plugin("codeql", 1, koptanv1.CIPluginSpec{Type: koptanv1.CIPluginCodeQL,
		CodeQL: &koptanv1.CodeQLConfig{BundleURL: "https://mirror.example.com/b.tgz"}})
	c, err = Step(&custom, BuildEnv{Language: "python"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEnv(c.Env, "CODEQL_BUNDLE_SHA256", "") {
		t.Errorf("an overridden bundle has no known digest: %+v", c.Env)
	}
}
