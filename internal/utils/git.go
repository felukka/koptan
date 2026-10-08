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

// ResolveRevision turns a branch, tag, full SHA or empty revision (the
// remote's default branch) into a commit SHA.
func ResolveRevision(_ context.Context, repo, revision, token string) (*Revision, error) {
	if IsSHA(revision) {
		return &Revision{SHA: revision, Ref: revision}, nil
	}

	refs, err := listRemoteRefs(repo, token)
	if err != nil {
		return nil, err
	}

	if revision == "" {
		var head *plumbing.Reference
		for _, ref := range refs {
			if ref.Name() == plumbing.HEAD {
				head = ref
			}
		}
		if head == nil {
			return nil, fmt.Errorf("%s has no default branch (HEAD)", repo)
		}
		if head.Type() == plumbing.HashReference {
			return &Revision{SHA: head.Hash().String(), Ref: string(plumbing.HEAD)}, nil
		}
		revision = head.Target().Short()
	}

	branchTarget := plumbing.NewBranchReferenceName(revision)
	tagTarget := plumbing.NewTagReferenceName(revision)
	peeled := tagTarget.String() + "^{}"
	var found *Revision
	for _, ref := range refs {
		switch ref.Name().String() {
		case branchTarget.String():
			return &Revision{SHA: ref.Hash().String(), Ref: string(ref.Name())}, nil
		case tagTarget.String():
			if found == nil {
				found = &Revision{SHA: ref.Hash().String(), Ref: string(ref.Name())}
			}
		case peeled:
			// Annotated tag: the peeled entry is the commit itself.
			found = &Revision{SHA: ref.Hash().String(), Ref: tagTarget.String()}
		}
	}
	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf("revision %q not found in %s", revision, repo)
}

// HasRevisionChanged resolves revision (branch, tag, SHA or empty) and
// reports whether it differs from knownSHA.
func HasRevisionChanged(ctx context.Context, repo, revision, knownSHA, token string) (bool, *Revision, error) {
	rev, err := ResolveRevision(ctx, repo, revision, token)
	if err != nil {
		return false, nil, err
	}
	return rev.SHA != knownSHA, rev, nil
}

func listRemoteRefs(repo, token string) ([]*plumbing.Reference, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repo},
	})

	var auth transport.AuthMethod
	if token != "" && IsHTTPURL(repo) {
		auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	return remote.List(&git.ListOptions{Auth: auth, PeelingOption: git.AppendPeeled})
}
