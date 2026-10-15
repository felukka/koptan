package utils

import (
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestTokenAuthSendsTokenOnlyOverHTTPS(t *testing.T) {
	auth, err := TokenAuth("https://git.example.com/r.git", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := auth.(*http.BasicAuth); !ok || b.Username != "x-access-token" || b.Password != "tok" {
		t.Errorf("https: auth = %#v", auth)
	}

	if _, err := TokenAuth("http://webhook-echo.tools.svc/org/repo.git", "tok"); err == nil ||
		!strings.Contains(err.Error(), "plain http") {
		t.Errorf("http with a token must be refused, got %v", err)
	}

	for _, repo := range []string{"http://git.example.com/r.git", "ssh://git@git.example.com/r.git"} {
		if auth, err := TokenAuth(repo, ""); err != nil || auth != nil {
			t.Errorf("%s without a token: auth = %#v, err = %v", repo, auth, err)
		}
	}
	if auth, err := TokenAuth("git@git.example.com:r.git", "tok"); err != nil || auth != nil {
		t.Errorf("scp-like: auth = %#v, err = %v", auth, err)
	}

	// listRemoteRefs must refuse before any network call.
	if _, err := ResolveRevision(t.Context(), "http://127.0.0.1:1/r.git", "main", "tok"); err == nil ||
		!strings.Contains(err.Error(), "plain http") {
		t.Errorf("ResolveRevision over http with a token: %v", err)
	}
}
