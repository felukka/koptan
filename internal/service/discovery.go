// Package service provides the discovery engine for koptan Service CRDs.
// It clones source repositories, detects the language, generates Dockerfiles
// when needed, and exposes results through a clean ServiceStatus.
package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// Engine orchestrates the full discovery lifecycle: clone, detect language,
// generate a Dockerfile if missing, and return results for the reconciler.
type Engine struct {
	Detectors []Detector
}

// NewEngine creates a discovery engine with the default language detectors
// registered in precedence order (most-specific first).
func NewEngine() *Engine {
	return &Engine{
		Detectors: []Detector{
			&GoDetector{},
			&JavaDetector{},
			&DotnetDetector{},
			&NodeDetector{},
			&PythonDetector{},
			&DockerfileOnlyDetector{},
		},
	}
}

// DiscoverResult holds the outcome of a discovery pass.
type DiscoverResult struct {
	Language      string
	HasDockerfile bool
	Dockerfile    []byte
	RootDir       string
}

// Discover clones the repository specified in the Service's Source and runs
// the language detection pipeline. It writes a generated Dockerfile if the
// repo has none and the detectors found a language.
func (e *Engine) Discover(ctx context.Context, svc *koptanv1.Service) (*DiscoverResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cloneDir, err := os.MkdirTemp("", "koptan.discover-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	token := ""
	if svc.Spec.Source.SecretRef != nil && svc.Spec.Source.SecretRef.Key != "" {
		// The reconciler is responsible for resolving the secret into an env var;
		// the engine reads it from the well-known environment variable.
		token = os.Getenv("KOPTAN_GIT_TOKEN")
	}

	err = e.cloneRepo(cloneDir, svc, token)
	if err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}

	revision := svc.Spec.Source.Revision
	if revision == "" {
		revision = "main"
	}
	if err := e.checkout(cloneDir, revision); err != nil {
		return nil, fmt.Errorf("checkout %s: %w", revision, err)
	}

	// Run detectors in registered order.
	language := ""
	for _, d := range e.Detectors {
		if d.Match(cloneDir) {
			language = d.Language()
			break
		}
	}
	if language == "" {
		return nil, fmt.Errorf("no supported language detected in %s", svc.Spec.Source.Repo)
	}

	// Check for existing Dockerfile.
	dockerfilePath := filepath.Join(cloneDir, "Dockerfile")
	// Also check root for Dockerfiles in common locations.
	dockerfilePath = filepath.Join(cloneDir, "Dockerfile")
	hasDockerfile := false
	var dockerfile []byte
	if data, err := os.ReadFile(dockerfilePath); err == nil {
		hasDockerfile = true
		dockerfile = data
	}

	// Generate if missing.
	if !hasDockerfile {
		dockerfile, err = e.generateDockerfile(cloneDir, language)
		if err != nil {
			return nil, fmt.Errorf("generate Dockerfile: %w", err)
		}
		// Write to the cloned workspace so the reconciler can read it.
		if err := os.WriteFile(dockerfilePath, dockerfile, 0o644); err != nil {
			return nil, fmt.Errorf("write Dockerfile: %w", err)
		}
	}

	return &DiscoverResult{
		Language:      language,
		HasDockerfile: hasDockerfile,
		Dockerfile:    dockerfile,
		RootDir:       cloneDir,
	}, nil
}

// cloneRepo clones the repository into the target directory.
func (e *Engine) cloneRepo(targetDir string, svc *koptanv1.Service, token string) error {
	url := svc.Spec.Source.Repo
	cloneConfig := &git.CloneOptions{
		URL:      url,
		Tags:     git.AllTags,
		Depth:    1,
		Progress: os.Stdout,
	}

	if token != "" {
		cloneConfig.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	_, err := git.PlainClone(targetDir, false, cloneConfig)
	return err
}

// checkout checks out the specified revision (branch, tag, or SHA).
func (e *Engine) checkout(repoDir, revision string) error {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	// Try as a branch name first.
	branchRef := plumbing.NewBranchReferenceName(revision)
	if _, err := repo.Reference(branchRef, true); err == nil {
		if err := wt.Checkout(&git.CheckoutOptions{Branch: branchRef, Force: true}); err != nil {
			// Fall through to tag/SHA.
		} else {
			return nil
		}
	}

	// Try as a tag.
	tagRef := plumbing.NewTagReferenceName(revision)
	if _, err := repo.Reference(tagRef, true); err == nil {
		if err := wt.Checkout(&git.CheckoutOptions{Branch: tagRef, Force: true}); err != nil {
			// Fall through to SHA.
		} else {
			return nil
		}
	}

	// Try as a commit SHA.
	hash := plumbing.NewHash(revision)
	if err := wt.Checkout(&git.CheckoutOptions{Hash: hash, Force: true}); err != nil {
		return fmt.Errorf("checkout revision %q: %w", revision, err)
	}
	return nil
}

// resolveRef looks up a reference by name. Returns true if found.
func resolveRef(repo *git.Repository, ref plumbing.ReferenceName) (*plumbing.Reference, error) {
	return repo.Reference(ref, true)
}

// generateDockerfile creates a minimal, secure Dockerfile for the detected language.
func (e *Engine) generateDockerfile(repoDir, language string) ([]byte, error) {
	tmpl, ok := languageDockerfiles[language]
	if !ok {
		return nil, fmt.Errorf("no Dockerfile template for language %q", language)
	}
	return []byte(tmpl), nil
}

// languageDockerfiles maps known languages to minimal, secure Dockerfile templates
// using multi-stage builds and non-root users.
var languageDockerfiles = map[string]string{
	"go": `# syntax=docker/dockerfile:1
# Stage 1: Build
FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app .

# Stage 2: Run
FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=builder /app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
`,
	"java": `# syntax=docker/dockerfile:1
# Stage 1: Build with Maven
FROM eclipse-temurin:21-jdk AS builder
WORKDIR /build
COPY pom.xml .
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn package -DskipTests -B

# Stage 2: Run
FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=builder /build/target/*.jar /app/app.jar
USER 1001:1001
ENTRYPOINT ["java", "-jar", "app.jar"]
`,
	"dotnet": `# syntax=docker/dockerfile:1
# Stage 1: Build
FROM mcr.microsoft.com/dotnet/sdk:8.0 AS builder
WORKDIR /src
COPY *.sln .
COPY **/*.csproj ./
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app --no-restore

# Stage 2: Run
FROM mcr.microsoft.com/dotnet/aspnet:8.0
WORKDIR /app
COPY --from=builder /app .
USER appuser
ENTRYPOINT ["dotnet", "app.dll"]
`,
	"node": `# syntax=docker/dockerfile:1
# Stage 1: Build
FROM node:20-bookworm AS builder
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY . .
RUN npm run build

# Stage 2: Run
FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=builder /build/package.json ./
RUN npm ci --omit=dev --omit=dev
COPY --from=builder /build/dist ./dist
USER node
CMD ["node", "dist/index.js"]
`,
	"python": `# syntax=docker/dockerfile:1
# Stage 1: Build
FROM python:3.12-slim AS builder
WORKDIR /build
COPY requirements.txt .
RUN pip install --user --no-cache-dir -r requirements.txt

# Stage 2: Run
FROM python:3.12-slim
WORKDIR /app
COPY --from=builder /root/.local /root/.local
COPY . .
ENV PATH=/root/.local/bin:$PATH
USER 1001:1001
CMD ["python", "-m", "app"]
`,
}
