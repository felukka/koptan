package steps

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	WorkspacePath      = "/workspace"
	DockerfilePath     = "/dockerfile"
	DockerConfigPath   = "/dockerconfig"
	DockerfileVolume   = "dockerfile-vol"
	DockerConfigVolume = "docker-config-vol"
	DockerfileKey      = "Dockerfile"
)

// Builder defines the interface for creating a step container
type Builder interface {
	Build() corev1.Container
}
