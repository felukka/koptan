// Package gitprovider creates repositories on git hosts for SelfServices.
// Each host is one file over net/http; EnsureRepo is idempotent, so an
// existing repository is returned as-is.
package gitprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RepoSpec describes the repository to ensure.
type RepoSpec struct {
	// Owner is an organization/user (GitHub) or group path (GitLab); empty
	// means the token's own account.
	Owner       string
	Name        string
	Private     bool
	Description string
}

// Provider creates repositories on one host.
type Provider interface {
	// EnsureRepo returns the https clone URL, creating the repo if needed.
	EnsureRepo(ctx context.Context, spec RepoSpec) (string, error)
}

// New returns the provider for kind ("github" or "gitlab"). baseURL is
// the API root for GitHub Enterprise or a self-hosted GitLab.
func New(kind, baseURL, token string, client *http.Client) (Provider, error) {
	if token == "" {
		return nil, errors.New("a token is required to create repositories")
	}
	if client == nil {
		client = http.DefaultClient
	}
	api := apiClient{token: token, client: client}
	switch kind {
	case "github":
		api.base = strings.TrimSuffix(firstNonEmpty(baseURL, "https://api.github.com"), "/")
		return github{api}, nil
	case "gitlab":
		api.base = strings.TrimSuffix(firstNonEmpty(baseURL, "https://gitlab.com"), "/") + "/api/v4"
		return gitlab{api}, nil
	}
	return nil, fmt.Errorf("unknown git provider %q", kind)
}

// errNotFound marks a 404 from the host API.
var errNotFound = errors.New("not found")

type apiClient struct {
	base   string
	token  string
	client *http.Client
	// header names the auth header; empty means Authorization: Bearer.
	header string
}

// do sends a JSON request and decodes a JSON answer into out.
func (a apiClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "koptan-operator")
	if a.header != "" {
		req.Header.Set(a.header, a.token)
	} else {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode >= 300:
		return fmt.Errorf("%s %s answered %d: %s", method, path, resp.StatusCode, apiMessage(data))
	case out != nil:
		return json.Unmarshal(data, out)
	}
	return nil
}

// apiMessage extracts the "message" of an error body, bounded.
func apiMessage(data []byte) string {
	var body struct {
		Message any `json:"message"`
	}
	if json.Unmarshal(data, &body) == nil && body.Message != nil {
		return fmt.Sprint(body.Message)
	}
	if len(data) > 200 {
		data = data[:200]
	}
	return string(bytes.TrimSpace(data))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
