// Package languages detects how to build a repository and renders a
// Dockerfile for it. Each language lives in its own file: a Language that
// inspects the repository and fills Facts, plus an embedded template under
// templates/<name>.Dockerfile.tmpl that turns those Facts into a Dockerfile.
//
// Adding a language means adding one file with a Language implementation,
// one template, and one entry in Registry.
package languages

import (
	"github.com/felukka/koptan/internal/service/repofs"
)

// Facts is what detection learned about a repository. Templates read it.
type Facts struct {
	// Language is the Language name, e.g. "go".
	Language string `json:"language"`
	// Version is the runtime/toolchain version used to pick base images.
	Version string `json:"version,omitempty"`
	// PackageManager is e.g. npm, yarn, pnpm, poetry, maven or gradle.
	PackageManager string `json:"packageManager,omitempty"`
	// Framework is a detected web framework, e.g. fastapi or rails.
	Framework string `json:"framework,omitempty"`
	// Entrypoint is what the image runs: a main package, module, jar or binary.
	Entrypoint string `json:"entrypoint,omitempty"`
	// InstallCmd installs dependencies in the build stage.
	InstallCmd string `json:"-"`
	// BuildCmd builds the application in the build stage; empty for none.
	BuildCmd string `json:"-"`
	// StartCmd is the exec-form command of the runtime image.
	StartCmd []string `json:"-"`
	// Port is the port the application listens on ($PORT).
	Port int32 `json:"-"`
	// Extra holds language-specific template values.
	Extra map[string]string `json:"-"`
}

// Language detects one ecosystem.
type Language interface {
	// Name is the language name and the template file prefix.
	Name() string
	// Detect inspects the build context. ok is false when the repository
	// is not this language; Facts are still returned with defaults so an
	// explicit spec.build.language can render them.
	Detect(fs *repofs.FS) (facts *Facts, ok bool)
}

// Registry lists the languages in detection order. More specific
// ecosystems come first: Rails, Django and Laravel apps often carry a
// package.json for assets, so node is tried late, and static last.
func Registry() []Language {
	return []Language{
		Go{}, Rust{}, Java{}, Dotnet{}, Python{}, Ruby{}, PHP{}, Node{}, Static{},
	}
}

// ByName returns the registered language called name.
func ByName(name string) (Language, bool) {
	for _, l := range Registry() {
		if l.Name() == name {
			return l, true
		}
	}
	return nil, false
}

// Names returns the registered language names, in detection order.
func Names() []string {
	registry := Registry()
	out := make([]string, 0, len(registry))
	for _, l := range registry {
		out = append(out, l.Name())
	}
	return out
}

// Detect returns the Facts of the first language that matches.
func Detect(fs *repofs.FS) (*Facts, bool) {
	for _, l := range Registry() {
		if facts, ok := l.Detect(fs); ok {
			return facts, true
		}
	}
	return nil, false
}

// newFacts starts Facts for a language with its default version.
func newFacts(language, version string) *Facts {
	return &Facts{Language: language, Version: version, Extra: map[string]string{}}
}
