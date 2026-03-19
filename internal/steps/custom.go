package steps

import (
	koptan "github.com/felukka/koptan/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
)

type Custom struct {
	Name string
	*koptan.CustomStep
}

func NewCustom(name string, step *koptan.CustomStep) *Custom {
	return &Custom{
		Name:       name,
		CustomStep: step,
	}
}

func (s *Custom) Build() corev1.Container {
	return corev1.Container{
		Name:         "step-" + s.Name,
		Image:        s.Image,
		Command:      s.Command,
		Args:         s.Args,
		Env:          s.Env,
		WorkingDir:   WorkspacePath,
		VolumeMounts: []corev1.VolumeMount{{Name: "workspace", MountPath: WorkspacePath}},
		Resources:    DefaultResources(),
	}
}
