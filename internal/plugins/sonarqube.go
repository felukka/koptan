package plugins

import (
	"errors"

	koptanv1 "github.com/felukka/koptan/api/v1"
	corev1 "k8s.io/api/core/v1"
)

const sonarScannerImage = "sonarsource/sonar-scanner-cli:11"

// SonarQube runs sonar-scanner on the source and, by default, waits for
// the project's quality gate so a failing gate fails the build.
type SonarQube struct{}

func (SonarQube) Validate(p *koptanv1.CIPlugin) error {
	cfg := p.Spec.SonarQube
	switch {
	case cfg == nil:
		return errors.New("spec.sonarqube is required for type sonarqube")
	case cfg.HostURL == "":
		return errors.New("spec.sonarqube.hostURL is required")
	case cfg.TokenSecretRef.Name == "" || cfg.TokenSecretRef.Key == "":
		return errors.New("spec.sonarqube.tokenSecretRef needs a name and a key")
	}
	return nil
}

func (SonarQube) Container(p *koptanv1.CIPlugin, env BuildEnv) (corev1.Container, error) {
	cfg := p.Spec.SonarQube
	image := cfg.Image
	if image == "" {
		image = sonarScannerImage
	}
	key := cfg.ProjectKey
	if key == "" {
		key = env.Namespace + "_" + env.Service
	}
	args := []string{
		"-Dsonar.projectKey=" + key,
		"-Dsonar.projectBaseDir=" + env.SourceDir(),
		"-Dsonar.scm.revision=" + env.Revision,
		// The workspace belongs to root; the scanner runs as a normal user.
		"-Dsonar.working.directory=/tmp/scannerwork",
	}
	if cfg.WaitForQualityGate == nil || *cfg.WaitForQualityGate {
		args = append(args, "-Dsonar.qualitygate.wait=true")
	}
	args = append(args, cfg.ExtraArgs...)
	return corev1.Container{
		Image:   image,
		Command: []string{"sonar-scanner"},
		Args:    args,
		Env: []corev1.EnvVar{
			{Name: "SONAR_HOST_URL", Value: cfg.HostURL},
			secretEnv("SONAR_TOKEN", cfg.TokenSecretRef),
			{Name: "SONAR_USER_HOME", Value: "/tmp/sonar"},
		},
	}, nil
}

func (SonarQube) Resources() corev1.ResourceRequirements {
	return requirements("200m", "512Mi", "2", "2Gi")
}
