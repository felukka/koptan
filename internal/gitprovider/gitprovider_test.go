package gitprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeHost serves a minimal GitHub or GitLab API from a route table and
// records requests.
type fakeHost struct {
	routes map[string]func(w http.ResponseWriter, body map[string]any)
	calls  []string
	auth   []string
}

func (f *fakeHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.EscapedPath()
	f.calls = append(f.calls, key)
	f.auth = append(f.auth, r.Header.Get("Authorization")+r.Header.Get("PRIVATE-TOKEN"))
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if h, ok := f.routes[key]; ok {
		h(w, body)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func reply(v any) func(http.ResponseWriter, map[string]any) {
	return func(w http.ResponseWriter, _ map[string]any) { _ = json.NewEncoder(w).Encode(v) }
}

func TestGitHubCreatesInAnOrgOnce(t *testing.T) {
	var created map[string]any
	host := &fakeHost{routes: map[string]func(http.ResponseWriter, map[string]any){
		"GET /user": reply(map[string]string{"login": "me"}),
		"POST /orgs/team/repos": func(w http.ResponseWriter, body map[string]any) {
			created = body
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"clone_url": "https://github.com/team/shop.git"})
		},
	}}
	srv := httptest.NewServer(host)
	defer srv.Close()
	p, err := New("github", srv.URL, "tok", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	url, err := p.EnsureRepo(context.Background(), RepoSpec{Owner: "team", Name: "shop", Private: true})
	if err != nil || url != "https://github.com/team/shop.git" {
		t.Fatalf("url %q, err %v", url, err)
	}
	if created["private"] != true || created["name"] != "shop" {
		t.Errorf("create body = %v", created)
	}
	if host.auth[0] != "Bearer tok" {
		t.Errorf("auth = %q", host.auth[0])
	}

	// Once it exists, it is reused.
	host.routes["GET /repos/team/shop"] = reply(map[string]string{"clone_url": "https://github.com/team/shop.git"})
	host.calls = nil
	if _, err := p.EnsureRepo(context.Background(), RepoSpec{Owner: "team", Name: "shop"}); err != nil {
		t.Fatal(err)
	}
	for _, c := range host.calls {
		if strings.HasPrefix(c, "POST") {
			t.Errorf("an existing repo must not be created again: %v", host.calls)
		}
	}
}

func TestGitHubUserRepo(t *testing.T) {
	host := &fakeHost{routes: map[string]func(http.ResponseWriter, map[string]any){
		"GET /user":        reply(map[string]string{"login": "me"}),
		"POST /user/repos": reply(map[string]string{"clone_url": "https://github.com/me/app.git"}),
	}}
	srv := httptest.NewServer(host)
	defer srv.Close()
	p, _ := New("github", srv.URL, "tok", srv.Client())
	if url, err := p.EnsureRepo(context.Background(), RepoSpec{Name: "app"}); err != nil || !strings.HasSuffix(url, "me/app.git") {
		t.Fatalf("url %q, err %v", url, err)
	}
}

func TestGitLabCreatesInAGroup(t *testing.T) {
	var created map[string]any
	host := &fakeHost{routes: map[string]func(http.ResponseWriter, map[string]any){
		"GET /api/v4/namespaces/team%2Fweb": reply(map[string]int{"id": 42}),
		"POST /api/v4/projects": func(w http.ResponseWriter, body map[string]any) {
			created = body
			_ = json.NewEncoder(w).Encode(map[string]string{"http_url_to_repo": "https://gitlab.example.com/team/web/shop.git"})
		},
	}}
	srv := httptest.NewServer(host)
	defer srv.Close()
	p, _ := New("gitlab", srv.URL, "glpat", srv.Client())
	url, err := p.EnsureRepo(context.Background(), RepoSpec{Owner: "team/web", Name: "shop", Private: true})
	if err != nil || url != "https://gitlab.example.com/team/web/shop.git" {
		t.Fatalf("url %q, err %v", url, err)
	}
	if created["namespace_id"] != float64(42) || created["visibility"] != "private" {
		t.Errorf("create body = %v", created)
	}
	if host.auth[0] != "glpat" {
		t.Errorf("GitLab uses PRIVATE-TOKEN, got %q", host.auth[0])
	}
}

func TestErrors(t *testing.T) {
	if _, err := New("github", "", "", nil); err == nil {
		t.Error("a token is required")
	}
	if _, err := New("bitbucket", "", "t", nil); err == nil {
		t.Error("unknown providers are rejected")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()
	p, _ := New("github", srv.URL, "bad", srv.Client())
	if _, err := p.EnsureRepo(context.Background(), RepoSpec{Name: "x"}); err == nil ||
		!strings.Contains(err.Error(), "Bad credentials") {
		t.Errorf("err = %v", err)
	}
}
