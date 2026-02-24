package utils

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

type Revision struct {
	SHA string
	Ref string
}

func ResolveBranch(_ context.Context, repo, branch, token string) (*Revision, error) {
	refs, err := listRemoteRefs(repo, token)
	if err != nil {
		return nil, err
	}

	target := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == target {
			return &Revision{
				SHA: ref.Hash().String(),
				Ref: string(target),
			}, nil
		}
	}

	return nil, fmt.Errorf("branch %q not found in %s", branch, repo)
}

func ResolveTag(_ context.Context, repo, tag, token string) (*Revision, error) {
	refs, err := listRemoteRefs(repo, token)
	if err != nil {
		return nil, err
	}

	target := plumbing.NewTagReferenceName(tag)
	for _, ref := range refs {
		if ref.Name() == target {
			return &Revision{
				SHA: ref.Hash().String(),
				Ref: string(target),
			}, nil
		}
	}

	return nil, fmt.Errorf("tag %q not found in %s", tag, repo)
}

func ResolveRevision(_ context.Context, repo, revision, token string) (*Revision, error) {
	if len(revision) >= 40 {
		return &Revision{SHA: revision, Ref: revision}, nil
	}

	refs, err := listRemoteRefs(repo, token)
	if err != nil {
		return nil, err
	}

	branchTarget := plumbing.NewBranchReferenceName(revision)
	tagTarget := plumbing.NewTagReferenceName(revision)

	for _, ref := range refs {
		if ref.Name() == branchTarget || ref.Name() == tagTarget {
			return &Revision{
				SHA: ref.Hash().String(),
				Ref: string(ref.Name()),
			}, nil
		}
	}

	return nil, fmt.Errorf("revision %q not found in %s", revision, repo)
}

func HasRevisionChanged(_ context.Context, repo, branch, knownSHA, token string) (bool, *Revision, error) {
	refs, err := listRemoteRefs(repo, token)
	if err != nil {
		return false, nil, err
	}

	target := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == target {
			rev := &Revision{
				SHA: ref.Hash().String(),
				Ref: string(target),
			}
			return rev.SHA != knownSHA, rev, nil
		}
	}

	return false, nil, fmt.Errorf("branch %q not found in %s", branch, repo)
}

func listRemoteRefs(repo, token string) ([]*plumbing.Reference, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repo},
	})

	var auth transport.AuthMethod
	if token != "" {
		auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	return remote.List(&git.ListOptions{Auth: auth})
}
