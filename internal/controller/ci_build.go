package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	koptanv1 "github.com/felukka/koptan/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	gitImage     = "alpine/git:2.47.2"
	buildahImage = "quay.io/buildah/stable:v1.43.0"

	workspacePath    = "/workspace"
	dockerfilePath   = "/koptan/dockerfile"
	dockerConfigPath = "/koptan/auth"

	labelCI       = "koptan.felukka.org/ci"
	labelRevision = "koptan.felukka.org/revision"
)

// The clone and build scripts never interpolate user input: the repository,
// revision and image arrive as environment variables and are only ever
// quoted, so nothing in them can become a shell command or a git option.
const cloneScript = `set -eu
url="$REPO"
if [ -n "${GIT_TOKEN:-}" ]; then
  case "$url" in
    https://*) url="https://x-access-token:${GIT_TOKEN}@${url#https://}" ;;
  esac
fi
git init -q "$WORKSPACE"
cd "$WORKSPACE"
git remote add origin "$url"
if ! git fetch -q --depth 1 origin "$SHA" 2>/dev/null; then
  # Servers that refuse to serve a bare SHA: fetch everything instead.
  git fetch -q origin
fi
git -c advice.detachedHead=false checkout -q --detach "$SHA"
`

const buildScript = `set -eu
context="$WORKSPACE/${CONTEXT_DIR:-.}"
if [ -f "$DOCKERFILE_DIR/.dockerignore" ] && [ ! -e "$context/.dockerignore" ]; then
  cp "$DOCKERFILE_DIR/.dockerignore" "$context/.dockerignore"
fi
buildah --storage-driver vfs build --isolation chroot \
  -f "$DOCKERFILE_DIR/Dockerfile" -t "$IMAGE" "$context"
if [ -f "$AUTH_DIR/config.json" ]; then
  buildah --storage-driver vfs push --authfile "$AUTH_DIR/config.json" "$IMAGE"
else
  buildah --storage-driver vfs push "$IMAGE"
fi
`

// imageRef is registry/repo:<first 12 chars of the commit SHA>.
func imageRef(ci *koptanv1.CI, sha string) string {
	registry := strings.TrimSuffix(ci.Spec.Registry.Registry, "/")
	repo := strings.Trim(ci.Spec.Registry.Repo, "/")
	return fmt.Sprintf("%s/%s:%s", registry, repo, short(sha))
}

// buildPod clones the repository at sha, builds it with the Dockerfile from
// the ConfigMap, and pushes image with the optional registry credentials.
func buildPod(ci *koptanv1.CI, svc *koptanv1.Service, sha, image, dockerCfgSecret string) *corev1.Pod {
	volumes := []corev1.Volume{
		{Name: "workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		// Every key is mounted: Dockerfile and, when generated, .dockerignore.
		{Name: "dockerfile", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: ci.Spec.DockerfileConfigMap},
		}}},
	}
	buildMounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: workspacePath},
		{Name: "dockerfile", MountPath: dockerfilePath, ReadOnly: true},
	}
	if dockerCfgSecret != "" {
		volumes = append(volumes, corev1.Volume{Name: "auth", VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: dockerCfgSecret,
				Items:      []corev1.KeyToPath{{Key: corev1.DockerConfigJsonKey, Path: "config.json"}},
			},
		}})
		buildMounts = append(buildMounts, corev1.VolumeMount{Name: "auth", MountPath: dockerConfigPath, ReadOnly: true})
	}

	cloneEnv := []corev1.EnvVar{
		{Name: "REPO", Value: svc.Spec.Source.Repo},
		{Name: "SHA", Value: sha},
		{Name: "WORKSPACE", Value: workspacePath},
	}
	if ref := svc.Spec.Source.SecretRef; ref != nil {
		key := ref.Key
		if key == "" {
			key = "token"
		}
		cloneEnv = append(cloneEnv, corev1.EnvVar{Name: "GIT_TOKEN", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: ref.LocalObjectReference, Key: key},
		}})
	}

	initContainers := []corev1.Container{{
		Name:                     "clone",
		Image:                    gitImage,
		Command:                  []string{"sh", "-c", cloneScript},
		Env:                      cloneEnv,
		VolumeMounts:             []corev1.VolumeMount{{Name: "workspace", MountPath: workspacePath}},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Resources:                resources("50m", "64Mi", "500m", "256Mi"),
	}}
	for _, extra := range ci.Spec.ExtraSteps {
		c := extra.DeepCopy()
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: "workspace", MountPath: workspacePath})
		initContainers = append(initContainers, *c)
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: ci.Name + "-build-",
			Namespace:    ci.Namespace,
			Labels: map[string]string{
				labelCI:       ci.Name,
				labelService:  svc.Name,
				labelRevision: short(sha),
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: initContainers,
			Containers: []corev1.Container{{
				Name:    "build-push",
				Image:   buildahImage,
				Command: []string{"sh", "-c", buildScript},
				Env: []corev1.EnvVar{
					{Name: "IMAGE", Value: image},
					{Name: "WORKSPACE", Value: workspacePath},
					{Name: "DOCKERFILE_DIR", Value: dockerfilePath},
					{Name: "CONTEXT_DIR", Value: ci.Spec.ContextDir},
					{Name: "AUTH_DIR", Value: dockerConfigPath},
				},
				VolumeMounts:             buildMounts,
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				Resources:                resources("500m", "512Mi", "2", "4Gi"),
			}},
			Volumes: volumes,
		},
	}
}

func resources(cpuReq, memReq, cpuLim, memLim string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuReq),
			corev1.ResourceMemory: resource.MustParse(memReq),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuLim),
			corev1.ResourceMemory: resource.MustParse(memLim),
		},
	}
}

// buildDockerConfigJSON renders a .dockerconfigjson for one registry.
func buildDockerConfigJSON(registry, username, password string) ([]byte, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return json.Marshal(map[string]any{
		"auths": map[string]any{
			registry: map[string]string{"username": username, "password": password, "auth": auth},
		},
	})
}

// podResult maps a build Pod to a CI phase and message; done reports
// whether the Pod has finished.
func podResult(pod *corev1.Pod) (phase koptanv1.CIPhase, msg string, done bool) {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return koptanv1.CIPhaseSucceeded, "build succeeded", true
	case corev1.PodFailed:
		msg = "build pod failed"
		if pod.Status.Message != "" {
			msg = pod.Status.Message
		}
		for _, c := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if t := c.State.Terminated; t != nil && t.ExitCode != 0 {
				detail := strings.TrimSpace(t.Message)
				if detail == "" {
					detail = t.Reason
				}
				if len(detail) > 1500 {
					detail = "…" + detail[len(detail)-1500:]
				}
				msg = fmt.Sprintf("%s failed (exit %d): %s", c.Name, t.ExitCode, detail)
				break
			}
		}
		return koptanv1.CIPhaseFailed, msg, true
	}
	msg = "build pod pending"
	if pod.Status.Phase == corev1.PodRunning {
		msg = "building"
	}
	for _, c := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if w := c.State.Waiting; w != nil && w.Reason != "" && w.Reason != "PodInitializing" {
			msg = fmt.Sprintf("%s: %s", c.Name, w.Reason)
			if w.Message != "" {
				msg += ": " + w.Message
			}
		}
	}
	return koptanv1.CIPhaseBuilding, msg, false
}
