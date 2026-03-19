package controller

import (
	"encoding/base64"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	koptan "github.com/felukka/koptan/api/v1alpha"
	"github.com/felukka/koptan/internal/steps"
)

func (r *SlipwayReconciler) buildPod(
	sw *koptan.Slipway,
	sha string,
	configMapName string,
	sourceRef koptan.SourceRef,
	imageTag string,
	dockerCfgSecret string,
) *corev1.Pod {
	volumes := buildVolumes(configMapName, dockerCfgSecret)

	authSecret := ""
	if sourceRef.PATToken != "" {
		authSecret = AuthSecretNameFor(sw.Spec.AppRef.Name)
	}

	initContainers := []corev1.Container{
		steps.NewCheckout(sourceRef.Repo, sha, authSecret).Build(),
	}

	for _, step := range sw.Spec.Steps {
		if b := steps.Factory(step); b != nil {
			initContainers = append(initContainers, b.Build())
		}
	}

	buildPushContainer := steps.NewBuildPush(imageTag, dockerCfgSecret).Build()

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: sw.Name + "-build-",
			Namespace:    sw.Namespace,
			Labels: map[string]string{
				"felukka.sh/slipway":   sw.Name,
				"felukka.sh/component": "slipway",
				"felukka.sh/function":  "ci",
				"felukka.sh/revision":  sha,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: initContainers,
			Containers:     []corev1.Container{buildPushContainer},
			Volumes:        volumes,
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
			Name: steps.DockerfileVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					Items: []corev1.KeyToPath{
						{Key: steps.DockerfileKey, Path: "Dockerfile"},
					},
				},
			},
		},
	}

	if dockerCfgSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: steps.DockerConfigVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: dockerCfgSecret,
					Items: []corev1.KeyToPath{
						{Key: ".dockerconfigjson", Path: ".config.json"},
					},
				},
			},
		})
	}
	return volumes
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
