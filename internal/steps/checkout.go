package steps

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const gitImage = "alpine/git:latest"

type Checkout struct {
	Repo       string
	Sha        string
	AuthSecret string
}

func NewCheckout(repo, sha, authSecret string) *Checkout {
	return &Checkout{
		Repo:       repo,
		Sha:        sha,
		AuthSecret: authSecret,
	}
}

func (s *Checkout) Build() corev1.Container {
	script := fmt.Sprintf(`set -e
REPO=%q
if [ -n "${GIT_TOKEN:-}" ]; then
    CLONE_URL=$(echo "$REPO" | sed "s|https://|https://x-access-token:${GIT_TOKEN}@|")
else
    CLONE_URL="$REPO"
fi
git init %s
cd %s
git remote add origin "$CLONE_URL"
git fetch --depth 1 origin %s
git checkout FETCH_HEAD
`, s.Repo, WorkspacePath, WorkspacePath, s.Sha)

	var env []corev1.EnvVar
	if s.AuthSecret != "" {
		env = append(env, corev1.EnvVar{
			Name: "GIT_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: s.AuthSecret},
					Key:                  "token",
				},
			},
		})
	}

	return corev1.Container{
		Name:    "step-checkout",
		Image:   gitImage,
		Command: []string{"sh", "-c", script},
		Env:     env,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: WorkspacePath},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}
}
