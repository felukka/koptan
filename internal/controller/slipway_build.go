package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	koptan "github.com/felukka/koptan/api/v1alpha"
)

func (r *SlipwayReconciler) buildJob(
	sw *koptan.Slipway,
	sha string,
	configMapName string,
	sourceRef koptan.SourceRef,
	imageTag string,
	dockerCfgSecret string,
) *batchv1.Job {
	var backoffLimit int32 = 0
	var ttlSeconds int32 = 600

	volumes := buildVolumes(configMapName, dockerCfgSecret)
	initContainers := r.buildInitContainers(sw, sourceRef, sha)
	buildPush := r.buildContainer(imageTag, dockerCfgSecret)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: sw.Name + "-build-",
			Namespace:    sw.Namespace,
			Labels: map[string]string{
				"felukka.sh/slipway":   sw.Name,
				"felukka.sh/component": "build",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"felukka.sh/slipway":   sw.Name,
						"felukka.sh/component": "build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers:     []corev1.Container{buildPush},
					Volumes:        volumes,
				},
			},
		},
	}
}

func buildVolumes(configMapName, dockerCfgSecret string) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name:         "workspace",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name: dockerfileVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					Items: []corev1.KeyToPath{
						{Key: dockerfileKey, Path: "Dockerfile"},
					},
				},
			},
		},
	}

	if dockerCfgSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: dockerConfigVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: dockerCfgSecret,
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: ".config.json",
						},
					},
				},
			},
		})
	}

	return volumes
}

func (r *SlipwayReconciler) buildInitContainers(
	sw *koptan.Slipway,
	sourceRef koptan.SourceRef,
	sha string,
) []corev1.Container {
	initContainers := make([]corev1.Container, 0, 1+len(sw.Spec.ExtraSteps))
	initContainers = append(initContainers, r.cloneContainer(sw.Spec.AppRef.Name, sourceRef, sha))

	for _, extra := range sw.Spec.ExtraSteps {
		c := extra.DeepCopy()
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      "workspace",
			MountPath: workspacePath,
		})
		initContainers = append(initContainers, *c)
	}

	return initContainers
}

func (r *SlipwayReconciler) cloneContainer(
	appName string,
	sourceRef koptan.SourceRef,
	sha string,
) corev1.Container {
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
`, sourceRef.Repo, workspacePath, workspacePath, sha)

	var env []corev1.EnvVar
	if sourceRef.PATToken != "" {
		authSecret := AuthSecretNameFor(appName)
		env = []corev1.EnvVar{
			{
				Name: "GIT_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: authSecret},
						Key:                  "token",
					},
				},
			},
		}
	}

	return corev1.Container{
		Name:    "clone",
		Image:   gitImage,
		Command: []string{"sh", "-c", script},
		Env:     env,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: workspacePath},
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

func (r *SlipwayReconciler) buildContainer(imageTag, dockerCfgSecret string) corev1.Container {
	script := buildahScript(imageTag, dockerCfgSecret)

	mounts := []corev1.VolumeMount{
		{Name: "workspace", MountPath: workspacePath},
		{Name: dockerfileVolume, MountPath: dockerfilePath, ReadOnly: true},
	}

	if dockerCfgSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      dockerConfigVolume,
			MountPath: dockerConfigPath,
			ReadOnly:  true,
		})
	}

	return corev1.Container{
		Name:         "build-push",
		Image:        buildahImage,
		Command:      []string{"sh", "-c", script},
		VolumeMounts: mounts,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
	}
}

func buildahScript(imageTag, dockerCfgSecret string) string {
	parts := []string{"set -e"}

	authFlag := ""
	if dockerCfgSecret != "" {
		authFlag = fmt.Sprintf("--authfile %s/.config.json", dockerConfigPath)
	}

	parts = append(parts, fmt.Sprintf(
		"buildah --storage-driver vfs build -f %s/Dockerfile -t %s %s",
		dockerfilePath, imageTag, workspacePath,
	))

	parts = append(parts, fmt.Sprintf(
		"buildah --storage-driver vfs push %s %s",
		authFlag, imageTag,
	))

	return strings.Join(parts, "\n")
}

func buildDockerConfigJSON(registry, username, password string) ([]byte, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	config := map[string]any{
		"auths": map[string]any{
			registry: map[string]string{
				"username": username,
				"password": password,
				"auth":     auth,
			},
		},
	}

	return json.Marshal(config)
}
