package plugins

import (
	"errors"

	koptanv1 "github.com/felukka/koptan/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// Custom runs any image against the source, e.g. trivy, gitleaks or a
// test suite. It starts in the build context and sees the KOPTAN_* env.
type Custom struct{}

func (Custom) Validate(p *koptanv1.CIPlugin) error {
	if p.Spec.Custom == nil || p.Spec.Custom.Image == "" {
		return errors.New("spec.custom.image is required for type custom")
	}
	return nil
}

func (Custom) Container(p *koptanv1.CIPlugin, _ BuildEnv) (corev1.Container, error) {
	cfg := p.Spec.Custom.DeepCopy()
	return corev1.Container{
		Image:   cfg.Image,
		Command: cfg.Command,
		Args:    cfg.Args,
		Env:     cfg.Env,
		EnvFrom: cfg.EnvFrom,
	}, nil
}

func (Custom) Resources() corev1.ResourceRequirements {
	return requirements("100m", "128Mi", "1", "1Gi")
}
