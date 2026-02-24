package gitprovider

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

type github struct{ api apiClient }

type githubRepo struct {
	CloneURL string `json:"clone_url"`
}

func (g github) EnsureRepo(ctx context.Context, spec RepoSpec) (string, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := g.api.do(ctx, http.MethodGet, "/user", nil, &user); err != nil {
		return "", err
	}
	owner := firstNonEmpty(spec.Owner, user.Login)

	var repo githubRepo
	err := g.api.do(ctx, http.MethodGet, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(spec.Name), nil, &repo)
	if err == nil {
		return repo.CloneURL, nil
	}
	if !errors.Is(err, errNotFound) {
		return "", err
	}

	path := "/user/repos"
	if owner != user.Login {
		path = "/orgs/" + url.PathEscape(owner) + "/repos"
	}
	body := map[string]any{"name": spec.Name, "private": spec.Private, "description": spec.Description}
	if err := g.api.do(ctx, http.MethodPost, path, body, &repo); err != nil {
		return "", err
	}
	return repo.CloneURL, nil
}
