package steps

import (
	corev1 "k8s.io/api/core/v1"
)

const sonarScannerImage = "sonarsource/sonar-scanner-cli:latest"

type SonarQube struct {
	Name   string
	Params map[string]string
}

func NewSonarQube(name string, params map[string]string) *SonarQube {
	return &SonarQube{
		Name:   name,
		Params: params,
	}
}

func (s *SonarQube) Build() corev1.Container {
	args := []string{
		"-Dsonar.host.url=" + s.Params["endpoint"],
		"-Dsonar.projectKey=" + s.Params["projectKey"],
		"-Dsonar.sources=.",
	}

	if s.Params["qualityGate"] == "true" {
		args = append(args, "-Dsonar.qualitygate.wait=true")
	}

	return corev1.Container{
		Name:         "step-" + s.Name,
		Image:        sonarScannerImage,
		Args:         args,
		WorkingDir:   WorkspacePath,
		VolumeMounts: []corev1.VolumeMount{{Name: "workspace", MountPath: WorkspacePath}},
		Env: []corev1.EnvVar{
			{
				Name: "SONAR_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: s.Params["tokenSecret"]},
						Key:                  "token",
					},
				},
			},
		},
		Resources: DefaultResources(),
	}
}
