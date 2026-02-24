package repofs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func setup(t *testing.T) *FS {
	t.Helper()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(repo, "app", "src"), 0o755))
	must(os.WriteFile(filepath.Join(repo, "app", "package.json"), []byte(`{"name":"x"}`), 0o644))
	must(os.WriteFile(filepath.Join(repo, "app", "src", "a.go"), []byte("package a"), 0o644))
	must(os.Symlink(filepath.Join(outside, "secret"), filepath.Join(repo, "leak")))
	must(os.Symlink(outside, filepath.Join(repo, "leakdir")))
	must(os.Symlink("app/package.json", filepath.Join(repo, "inside")))
	fs, err := New(repo)
	must(err)
	return fs
}

func TestStaysInsideTheRepository(t *testing.T) {
	fs := setup(t)
	for _, p := range []string{"leak", "leakdir/secret", "../" + filepath.Base(fs.Root()), "/etc/passwd"} {
		if data, err := fs.Read(p); err == nil && string(data) == "s3cret" {
			t.Errorf("%s: read a file outside the repository", p)
		}
	}
	if _, err := fs.Read("leak"); !errors.Is(err, ErrOutside) {
		t.Errorf("symlink out: err = %v, want ErrOutside", err)
	}
	if _, err := fs.Sub("leakdir"); !errors.Is(err, ErrOutside) {
		t.Errorf("sub through symlink: err = %v, want ErrOutside", err)
	}
	// "/etc/passwd" and "../x" are clamped to the root, not followed out.
	if fs.Exists("/etc/passwd") {
		t.Error("absolute paths must resolve inside the repository")
	}
	if got := fs.ReadString("inside"); got != `{"name":"x"}` {
		t.Errorf("symlink inside the repository: got %q", got)
	}
}

func TestHelpers(t *testing.T) {
	fs := setup(t)
	sub, err := fs.Sub("app")
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct{ Name string }
	if err := sub.ReadJSON("package.json", &pkg); err != nil || pkg.Name != "x" {
		t.Errorf("ReadJSON: %v %+v", err, pkg)
	}
	if got := sub.Glob("src", "*.go"); len(got) != 1 || got[0] != "src/a.go" {
		t.Errorf("Glob = %v", got)
	}
	if got := sub.List(".", true); len(got) != 1 || got[0] != "src" {
		t.Errorf("List dirs = %v", got)
	}
	if !sub.IsDir("src") || sub.IsDir("package.json") {
		t.Error("IsDir")
	}
	if got := sub.FirstExisting("nope", "package.json"); got != "package.json" {
		t.Errorf("FirstExisting = %q", got)
	}
}

func TestReadIsBounded(t *testing.T) {
	dir := t.TempDir()
	big := make([]byte, MaxFileSize+1)
	if err := os.WriteFile(filepath.Join(dir, "big"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Read("big"); err == nil {
		t.Error("want an error for a file over MaxFileSize")
	}
}

func TestTOML(t *testing.T) {
	doc := `name = "top"
[package]
name = "hello" # trailing comment
edition = '2021'
[tool.poetry]
version = 3
`
	cases := map[[2]string]string{
		{"", "name"}:               "top",
		{"package", "name"}:        "hello",
		{"package", "edition"}:     "2021",
		{"tool.poetry", "version"}: "3",
		{"package", "missing"}:     "",
	}
	for in, want := range cases {
		if got := TOMLValue(doc, in[0], in[1]); got != want {
			t.Errorf("TOMLValue(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
	if !TOMLHasSection(doc, "tool.poetry") || TOMLHasSection(doc, "tool") {
		t.Error("TOMLHasSection")
	}
}
