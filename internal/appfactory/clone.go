package appfactory

import (
	"fmt"
	"net/url"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type CloneOptions struct {
	Repo     string
	Revision string
	Token    string
	Dir      string
}

func Checkout(opts CloneOptions) (string, error) {
	if !isValidGitURL(opts.Repo) {
		return "", fmt.Errorf("invalid git url: %s", opts.Repo)
	}

	if opts.Dir == "" {
		dir, err := os.MkdirTemp("", "koptan-clone-*")
		if err != nil {
			return "", fmt.Errorf("creating temp dir: %w", err)
		}
		opts.Dir = dir
	}

	cloneOpts := &git.CloneOptions{
		URL:   opts.Repo,
		Depth: 1,
	}

	if opts.Token != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: opts.Token,
		}
	}

	if opts.Revision != "" {
		if len(opts.Revision) >= 40 {
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName("HEAD")
		} else {
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Revision)
		}
	}

	repo, err := git.PlainClone(opts.Dir, false, cloneOpts)
	if err != nil {
		return "", fmt.Errorf("cloning %s: %w", opts.Repo, err)
	}

	if len(opts.Revision) >= 40 {
		wt, err := repo.Worktree()
		if err != nil {
			return "", fmt.Errorf("getting worktree: %w", err)
		}
		err = wt.Checkout(&git.CheckoutOptions{
			Hash: plumbing.NewHash(opts.Revision),
		})
		if err != nil {
			return "", fmt.Errorf("checking out %s: %w", opts.Revision, err)
		}
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("getting HEAD: %w", err)
	}

	return head.Hash().String(), nil
}

func isValidGitURL(repoURL string) bool {
	if repoURL == "" {
		return false
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return false
	}

	if u.Host == "" || u.Path == "" {
		return false
	}

	return true
}
