package gitprovider

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

type gitlab struct{ api apiClient }

type gitlabProject struct {
	HTTPURL string `json:"http_url_to_repo"`
}

func (g gitlab) EnsureRepo(ctx context.Context, spec RepoSpec) (string, error) {
	g.api.header = "PRIVATE-TOKEN"
	owner := spec.Owner
	if owner == "" {
		var user struct {
			Username string `json:"username"`
		}
		if err := g.api.do(ctx, http.MethodGet, "/user", nil, &user); err != nil {
			return "", err
		}
		owner = user.Username
	}

	var project gitlabProject
	err := g.api.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(owner+"/"+spec.Name), nil, &project)
	if err == nil {
		return project.HTTPURL, nil
	}
	if !errors.Is(err, errNotFound) {
		return "", err
	}

	visibility := "public"
	if spec.Private {
		visibility = "private"
	}
	body := map[string]any{"name": spec.Name, "path": spec.Name, "visibility": visibility,
		"description": spec.Description}
	if spec.Owner != "" {
		var ns struct {
			ID int64 `json:"id"`
		}
		if err := g.api.do(ctx, http.MethodGet, "/namespaces/"+url.PathEscape(spec.Owner), nil, &ns); err != nil {
			return "", err
		}
		body["namespace_id"] = ns.ID
	}
	if err := g.api.do(ctx, http.MethodPost, "/projects", body, &project); err != nil {
		return "", err
	}
	return project.HTTPURL, nil
}
