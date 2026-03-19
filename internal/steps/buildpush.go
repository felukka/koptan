package steps

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const buildahImage = "quay.io/buildah/stable:latest"

type BuildPush struct {
	ImageTag        string
	DockerCfgSecret string
}

func NewBuildPush(imageTag, dockerCfgSecret string) *BuildPush {
	return &BuildPush{
		ImageTag:        imageTag,
		DockerCfgSecret: dockerCfgSecret,
	}
}

func (s *BuildPush) Build() corev1.Container {
	script := s.buildScript()

	mounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: WorkspacePath},
		{Name: DockerfileVolume, MountPath: DockerfilePath, ReadOnly: true},
	}

	if s.DockerCfgSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      DockerConfigVolume,
			MountPath: DockerConfigPath,
			ReadOnly:  true,
		})
	}

	return corev1.Container{
		Name:         "step-build-push",
		Image:        buildahImage,
		Command:      []string{"sh", "-c", script},
		VolumeMounts: mounts,
		Resources:    LargeResources(),
		SecurityContext: &corev1.SecurityContext{
			Privileged: func(b bool) *bool { return &b }(true),
		},
	}
}

func (s *BuildPush) buildScript() string {
	parts := []string{"set -e"}

	authFlag := ""
	if s.DockerCfgSecret != "" {
		authFlag = fmt.Sprintf("--authfile %s/.config.json", DockerConfigPath)
	}

	parts = append(parts, fmt.Sprintf(
		"buildah --storage-driver vfs build -f %s/%s -t %s %s",
		DockerfilePath, DockerfileKey, s.ImageTag, WorkspacePath,
	))

	parts = append(parts, fmt.Sprintf(
		"buildah --storage-driver vfs push %s %s",
		authFlag, s.ImageTag,
	))

	return strings.Join(parts, "\n")
}
