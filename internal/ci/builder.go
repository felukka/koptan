package ci

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// Builder is the interface for container image build backends.
// Implementations can use buildah, kaniko, buildpacks, or external CI systems.
type Builder interface {
	// Build clones the service source, builds the image using the Dockerfile,
	// and pushes it to the registry. Returns the full image reference on success.
	Build(ctx context.Context, ci *koptanv1.CI, svc *koptanv1.Service) (string, error)
}

// BuildahBuilder implements Builder using the buildah CLI tool.
// This is the default backend; swap in Kaniko or Buildpacks by
// changing the builder registration in the CI controller.
type BuildahBuilder struct{}

var _ Builder = (*BuildahBuilder)(nil)

// Build clones the service, builds with buildah, and pushes to the target registry.
func (b *BuildahBuilder) Build(ctx context.Context, ci *koptanv1.CI, svc *koptanv1.Service) (string, error) {
	// Resolve the git token from the secret if provided.
	token := ""
	if svc.Spec.Source.SecretRef != nil {
		secretPath := os.Getenv(fmt.Sprintf("KOPTAN_GIT_TOKEN_%s", ci.Namespace))
		token = secretPath
	}

	// Clone the repo.
	cloneDir, err := os.MkdirTemp("", "koptan.ci-build-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", svc.Spec.Source.Repo, cloneDir)
	if token != "" {
		// Use git credential helper for authenticated clones.
		cloneCmd.Env = append(os.Environ(),
			fmt.Sprintf("GIT_TERMINAL_PROMPT=0"),
		)
		// Create a temporary credentials file.
		credPath := filepath.Join(cloneDir, ".git-cred")
		if err := os.WriteFile(credPath, []byte(svc.Spec.Source.Repo+"\nx-access-token:"+token), 0o600); err != nil {
			return "", fmt.Errorf("write cred file: %w", err)
		}
	}

	if out, err := cloneCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", string(out), err)
	}

	// Checkout revision.
	revision := svc.Spec.Source.Revision
	if revision == "" {
		revision = "main"
	}
	checkoutCmd := exec.CommandContext(ctx, "git", "-C", cloneDir, "checkout", revision)
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git checkout failed: %s: %w", string(out), err)
	}

	// Build with buildah.
	imageTag := fmt.Sprintf("%s/%s/%s:%s",
		ci.Spec.Registry.Registry,
		ci.Spec.Registry.Repo,
		svc.Name,
		revision[:12],
	)

	buildCmd := exec.CommandContext(ctx, "buildah", "build",
		"-t", imageTag,
		"-f", filepath.Join(cloneDir, "Dockerfile"),
		cloneDir,
	)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("buildah build failed: %s: %w", string(out), err)
	}

	// Push to registry.
	pushCmd := exec.CommandContext(ctx, "buildah", "push", imageTag, "docker://"+imageTag)
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("buildah push failed: %s: %w", string(out), err)
	}

	return imageTag, nil
}
