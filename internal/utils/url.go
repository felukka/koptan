package utils

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	scpLikeURL = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._/~-]+$`)
	revisionRe = regexp.MustCompile(`^[A-Za-z0-9._/][A-Za-z0-9._/-]*$`)
	shaRe      = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// ValidateGitURL accepts only remote git URLs (https, http, ssh, git, or
// scp-like user@host:path). It rejects local paths, file://, ext:: and
// anything git could read as a command-line option.
func ValidateGitURL(repo string) error {
	if repo == "" {
		return fmt.Errorf("repository URL is empty")
	}
	if strings.HasPrefix(repo, "-") || strings.ContainsAny(repo, " \t\r\n") {
		return fmt.Errorf("repository URL %q is not allowed", repo)
	}
	if scpLikeURL.MatchString(repo) {
		return nil
	}
	u, err := url.Parse(repo)
	if err != nil {
		return fmt.Errorf("repository URL %q: %w", repo, err)
	}
	switch u.Scheme {
	case "https", "http", "ssh", "git":
	default:
		return fmt.Errorf("repository URL scheme %q is not allowed (use https, http, ssh or git)", u.Scheme)
	}
	if u.Host == "" || u.Path == "" || u.Path == "/" {
		return fmt.Errorf("repository URL %q needs a host and a path", repo)
	}
	return nil
}

// ValidateRevision accepts an empty revision (default branch), a branch,
// a tag or a commit SHA, but never something that looks like an option.
func ValidateRevision(rev string) error {
	if rev == "" {
		return nil
	}
	if len(rev) > 250 || !revisionRe.MatchString(rev) || strings.Contains(rev, "..") {
		return fmt.Errorf("revision %q is not allowed", rev)
	}
	return nil
}

// IsSHA reports whether rev is a full 40-character commit SHA.
func IsSHA(rev string) bool { return shaRe.MatchString(rev) }

// IsHTTPURL reports whether a token can be sent as HTTP basic auth.
func IsHTTPURL(repo string) bool {
	return strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "http://")
}

var relPathRe = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)

// ValidateRelPath accepts an empty path or a relative path inside the
// repository: no leading slash, no "." or ".." segments.
func ValidateRelPath(field, p string) error {
	if p == "" {
		return nil
	}
	if len(p) > 250 || !relPathRe.MatchString(p) {
		return fmt.Errorf("%s %q is not a relative path", field, p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." {
			return fmt.Errorf("%s %q must not contain . or .. segments", field, p)
		}
	}
	return nil
}
