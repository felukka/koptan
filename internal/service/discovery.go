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

	"github.com/felukka/koptan/internal/utils"
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
}

// Discover clones the repository at rev, detects the language and returns
// the repository's own Dockerfile or a generated one. The token is only
// sent to http(s) remotes.
func (e *Engine) Discover(ctx context.Context, repo string, rev *utils.Revision, token string) (*DiscoverResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cloneDir, err := os.MkdirTemp("", "koptan.discover-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(cloneDir) }()

	if err := cloneAt(ctx, cloneDir, repo, rev, token); err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}

	language := ""
	for _, d := range e.Detectors {
		if d.Match(cloneDir) {
			language = d.Language()
			break
		}
	}
	if language == "" {
		return nil, fmt.Errorf("no supported language detected in %s", repo)
	}

	result := &DiscoverResult{Language: language}
	if data, err := os.ReadFile(filepath.Join(cloneDir, "Dockerfile")); err == nil {
		result.HasDockerfile = true
		result.Dockerfile = data
		return result, nil
	}
	tmpl, ok := languageDockerfiles[language]
	if !ok {
		return nil, fmt.Errorf("no Dockerfile template for language %q", language)
	}
	result.Dockerfile = []byte(tmpl)
	return result, nil
}

// cloneAt checks out exactly rev: a shallow single-branch clone for a
// branch or tag, a full clone plus checkout for a bare commit SHA.
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

// languageDockerfiles are used when a repository has no Dockerfile. Every
// image listens on $PORT, which the CD sets (default 8080).
var languageDockerfiles = map[string]string{
	"go": `# syntax=docker/dockerfile:1
FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app /app
USER nonroot:nonroot
ENV PORT=8080
ENTRYPOINT ["/app"]
`,
	"java": `# syntax=docker/dockerfile:1
FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /build
COPY pom.xml .
RUN mvn -B dependency:go-offline
COPY src ./src
RUN mvn -B package -DskipTests && cp "$(ls target/*.jar | grep -v -- '-sources\|-javadoc\|original-' | head -n1)" /app.jar

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=builder /app.jar /app/app.jar
USER 1001:1001
ENV PORT=8080
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
`,
	"dotnet": `# syntax=docker/dockerfile:1
FROM mcr.microsoft.com/dotnet/sdk:8.0 AS builder
WORKDIR /src
COPY . .
RUN dotnet publish -c Release -o /app -p:AssemblyName=app

FROM mcr.microsoft.com/dotnet/aspnet:8.0
WORKDIR /app
COPY --from=builder /app .
USER app
ENV PORT=8080 ASPNETCORE_HTTP_PORTS=8080
ENTRYPOINT ["dotnet", "app.dll"]
`,
	"node": `# syntax=docker/dockerfile:1
FROM node:20-bookworm AS builder
WORKDIR /build
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY . .
RUN npm run build --if-present

FROM node:20-bookworm-slim
WORKDIR /app
ENV NODE_ENV=production PORT=8080
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci --omit=dev; else npm install --omit=dev; fi
COPY --from=builder /build .
USER node
CMD ["npm", "start"]
`,
	"python": `# syntax=docker/dockerfile:1
FROM python:3.12-slim AS builder
WORKDIR /build
COPY requirements.txt* pyproject.toml* ./
RUN mkdir -p /install && if [ -f requirements.txt ]; then pip install --no-cache-dir --prefix=/install -r requirements.txt; \
    else pip install --no-cache-dir --prefix=/install .; fi

FROM python:3.12-slim
WORKDIR /app
COPY --from=builder /install /usr/local
COPY . .
USER 1001:1001
ENV PORT=8080
CMD ["python", "-m", "app"]
`,
}
