// Package service provides the discovery engine for koptan Service CRDs.
// It clones the source repository at an exact revision, detects the stack,
// and returns the repository's own Dockerfile or one generated for it.
package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/felukka/koptan/internal/service/languages"
	"github.com/felukka/koptan/internal/service/repofs"
	"github.com/felukka/koptan/internal/utils"
)

// Dockerfile sources recorded in the Service status.
const (
	SourceRepo     = "repo"
	SourceTemplate = "template"
)

// LanguageDockerfile is reported when a repository brings its own
// Dockerfile and no known stack was detected.
const LanguageDockerfile = "dockerfile"

// Request says what to discover.
type Request struct {
	Repo  string
	Rev   *utils.Revision
	Token string
	// ContextDir is the build context relative to the repository root.
	ContextDir string
	// DockerfilePath is an explicit Dockerfile, relative to the root.
	DockerfilePath string
	// Language skips detection when set.
	Language string
	// Port the application listens on; generated images export it as $PORT.
	Port int32
}

// Result is the outcome of a discovery pass.
type Result struct {
	// Facts describe the detected stack; nil when only a repository
	// Dockerfile was found.
	Facts *languages.Facts
	// Source is SourceRepo or SourceTemplate.
	Source     string
	Dockerfile []byte
	// Dockerignore is set for generated Dockerfiles when the build context
	// has no .dockerignore of its own.
	Dockerignore []byte
}

// Language is the detected language, or LanguageDockerfile.
func (r *Result) Language() string {
	if r.Facts == nil {
		return LanguageDockerfile
	}
	return r.Facts.Language
}

// Engine runs discovery. The zero value is ready to use.
type Engine struct{}

// NewEngine returns a discovery engine.
func NewEngine() *Engine { return &Engine{} }

// Discover clones the repository at req.Rev and works out its Dockerfile.
func (e *Engine) Discover(ctx context.Context, req Request) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cloneDir, err := os.MkdirTemp("", "koptan.discover-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(cloneDir) }()

	if err := cloneAt(ctx, cloneDir, req.Repo, req.Rev, req.Token); err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	root, err := repofs.New(cloneDir)
	if err != nil {
		return nil, err
	}
	return e.DiscoverDir(root, req)
}

// DiscoverDir runs discovery on an already checked-out repository.
func (e *Engine) DiscoverDir(root *repofs.FS, req Request) (*Result, error) {
	buildCtx := root
	if req.ContextDir != "" {
		sub, err := root.Sub(req.ContextDir)
		if err != nil {
			return nil, fmt.Errorf("spec.build.contextDir %q: %w", req.ContextDir, err)
		}
		buildCtx = sub
	}

	facts, err := detect(buildCtx, req.Language)
	if err != nil {
		return nil, err
	}

	result := &Result{Facts: facts}
	own, found, err := locateDockerfile(root, buildCtx, req.DockerfilePath)
	switch {
	case err != nil:
		return nil, err
	case found:
		result.Source = SourceRepo
		result.Dockerfile = own
	case facts == nil:
		return nil, fmt.Errorf("no Dockerfile and no supported stack found in %s (supported: %s)",
			describeContext(req), strings.Join(languages.Names(), ", "))
	default:
		facts.Port = req.Port
		rendered, err := languages.Render(facts)
		if err != nil {
			return nil, err
		}
		result.Source = SourceTemplate
		result.Dockerfile = rendered
		if !buildCtx.Exists(".dockerignore") {
			result.Dockerignore = []byte(defaultDockerignore)
		}
	}
	if err := ValidateDockerfile(result.Dockerfile); err != nil {
		return nil, err
	}
	return result, nil
}

// detect returns the Facts of the requested or detected language, or nil
// when nothing matches and no language was requested.
func detect(fs *repofs.FS, language string) (*languages.Facts, error) {
	if language == "" {
		facts, _ := languages.Detect(fs)
		return facts, nil
	}
	lang, ok := languages.ByName(language)
	if !ok {
		return nil, fmt.Errorf("spec.build.language %q is not supported (supported: %s)",
			language, strings.Join(languages.Names(), ", "))
	}
	facts, _ := lang.Detect(fs)
	return facts, nil
}

func describeContext(req Request) string {
	if req.ContextDir == "" {
		return req.Repo
	}
	return req.Repo + " (" + req.ContextDir + ")"
}
