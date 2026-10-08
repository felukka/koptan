package utils

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGitURL(t *testing.T) {
	ok := []string{
		"https://github.com/felukka/koptan.git",
		"http://git.local/team/app",
		"ssh://git@github.com/felukka/koptan.git",
		"git://172.18.0.1:9418/hello.git",
		"git@github.com:felukka/koptan.git",
	}
	for _, u := range ok {
		if err := ValidateGitURL(u); err != nil {
			t.Errorf("ValidateGitURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{
		"", "--upload-pack=touch /tmp/pwned", "file:///etc", "/srv/repo.git",
		"ext::sh -c touch% /tmp/x", "https://", "https://host", "ftp://host/repo",
		"https://github.com/a b",
	}
	for _, u := range bad {
		if err := ValidateGitURL(u); err == nil {
			t.Errorf("ValidateGitURL(%q) = nil, want error", u)
		}
	}
}

func TestValidateRevision(t *testing.T) {
	for _, r := range []string{"", "main", "feature/x", "v1.0.0", strings.Repeat("a", 40)} {
		if err := ValidateRevision(r); err != nil {
			t.Errorf("ValidateRevision(%q) = %v", r, err)
		}
	}
	for _, r := range []string{"--orphan", "-x", "a b", "a..b", "main;rm", strings.Repeat("a", 251)} {
		if err := ValidateRevision(r); err == nil {
			t.Errorf("ValidateRevision(%q) = nil, want error", r)
		}
	}
}

// TestResolveRevision runs against a local bare repository with a
// non-"main" default branch, a second branch and an annotated tag.
func TestResolveRevision(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	work, bare := filepath.Join(dir, "work"), filepath.Join(dir, "repo.git")
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := exec.Command("git", "init", "-q", "-b", "master", work).Run(); err != nil {
		t.Fatal(err)
	}
	run("commit", "-q", "--allow-empty", "-m", "one")
	master := run("rev-parse", "HEAD")
	run("tag", "-a", "v1", "-m", "v1")
	run("checkout", "-q", "-b", "dev")
	run("commit", "-q", "--allow-empty", "-m", "two")
	dev := run("rev-parse", "HEAD")
	run("checkout", "-q", "master")
	if out, err := exec.Command("git", "clone", "-q", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v %s", err, out)
	}

	ctx := context.Background()
	cases := map[string]string{"": master, "master": master, "dev": dev, "v1": master, dev: dev}
	for rev, want := range cases {
		got, err := ResolveRevision(ctx, bare, rev, "")
		if err != nil {
			t.Errorf("ResolveRevision(%q): %v", rev, err)
			continue
		}
		if got.SHA != want {
			t.Errorf("ResolveRevision(%q) = %s, want %s", rev, got.SHA, want)
		}
	}
	if _, err := ResolveRevision(ctx, bare, "nope", ""); err == nil {
		t.Error("ResolveRevision(nope) = nil error")
	}
	changed, _, err := HasRevisionChanged(ctx, bare, "dev", dev, "")
	if err != nil || changed {
		t.Errorf("HasRevisionChanged(dev, same) = %v, %v", changed, err)
	}
}
