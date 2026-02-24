package service

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/felukka/koptan/internal/utils"
)

// cloneAt checks out exactly rev: a shallow single-branch clone for a
// branch or tag, a full clone plus checkout for a bare commit SHA. The
// token is only sent to http(s) remotes.
func cloneAt(ctx context.Context, dir, repo string, rev *utils.Revision, token string) error {
	opts := &git.CloneOptions{URL: repo}
	if token != "" && utils.IsHTTPURL(repo) {
		opts.Auth = &http.BasicAuth{Username: "x-access-token", Password: token}
	}
	ref := plumbing.ReferenceName(rev.Ref)
	if ref.IsBranch() || ref.IsTag() {
		opts.ReferenceName = ref
		opts.SingleBranch = true
		opts.Depth = 1
		_, err := git.PlainCloneContext(ctx, dir, false, opts)
		return err
	}
	opts.NoCheckout = true
	r, err := git.PlainCloneContext(ctx, dir, false, opts)
	if err != nil {
		return err
	}
	wt, err := r.Worktree()
	if err != nil {
		return err
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(rev.SHA), Force: true}); err != nil {
		return fmt.Errorf("checkout %s: %w", rev.SHA, err)
	}
	return nil
}
