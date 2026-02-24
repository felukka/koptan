// Package plugins turns CIPlugins into build steps. A step is an init
// container of the build Pod that runs after the clone and before
// build/push, with the checked-out source mounted at WorkspacePath.
//
// Each plugin type has a Renderer in its own file. Adding a tool means
// adding one Renderer and one entry in renderers; the CRD enum in
// api/v1/ciplugin_types.go lists the types the API accepts.
package plugins

import (
	"fmt"

	koptanv1 "github.com/felukka/koptan/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// Volumes and paths the build Pod provides to every step.
const (
	WorkspaceVolume = "workspace"
	WorkspacePath   = "/workspace"
	ReportsVolume   = "reports"
	ReportsPath     = "/koptan/reports"
	containerPrefix = "plugin-"
)

// BuildEnv describes the build a step runs in.
type BuildEnv struct {
	Service    string
	Namespace  string
	Repo       string
	Revision   string
	Language   string
	ContextDir string
}

// SourceDir is the checked-out build context inside a step.
func (e BuildEnv) SourceDir() string {
	if e.ContextDir == "" {
		return WorkspacePath
	}
	return WorkspacePath + "/" + e.ContextDir
}

// Renderer builds the container of one plugin type. Steps adds the name,
// mounts, common environment, resources and failure policy.
type Renderer interface {
	// Validate checks the type-specific configuration.
	Validate(p *koptanv1.CIPlugin) error
	// Container returns the step container.
	Container(p *koptanv1.CIPlugin, env BuildEnv) (corev1.Container, error)
	// Resources are the defaults when spec.resources is unset.
	Resources() corev1.ResourceRequirements
}

var renderers = map[koptanv1.CIPluginType]Renderer{
	koptanv1.CIPluginSonarQube: SonarQube{},
	koptanv1.CIPluginCodeQL:    CodeQL{},
	koptanv1.CIPluginCustom:    Custom{},
}

func rendererFor(p *koptanv1.CIPlugin) (Renderer, error) {
	r, ok := renderers[p.Spec.Type]
	if !ok {
		return nil, fmt.Errorf("unknown plugin type %q", p.Spec.Type)
	}
	return r, nil
}

// Validate checks a plugin's configuration before it is used.
func Validate(p *koptanv1.CIPlugin) error {
	r, err := rendererFor(p)
	if err != nil {
		return err
	}
	if err := r.Validate(p); err != nil {
		return err
	}
	if p.Spec.FailurePolicy == koptanv1.FailurePolicyIgnore {
		c, err := r.Container(p, BuildEnv{})
		if err != nil {
			return err
		}
		if len(c.Command) == 0 {
			return fmt.Errorf("failurePolicy Ignore needs an explicit command")
		}
	}
	return nil
}
