package controller

import (
	"strings"
	"testing"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/service"
	"github.com/felukka/koptan/internal/service/languages"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testCI() *koptanv1.CI {
	return &koptanv1.CI{
		ObjectMeta: metav1.ObjectMeta{Name: "web-ci", Namespace: "default"},
		Spec: koptanv1.CISpec{
			Service:             koptanv1.NamespacedObjectReference{Name: "web"},
			Registry:            koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "team/web"},
			Revision:            "0123456789abcdef0123456789abcdef01234567",
			DockerfileConfigMap: "web-dockerfile-abcd",
			ContextDir:          "services/web",
		},
	}
}

func testService() *koptanv1.Service {
	return &koptanv1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/web.git"}},
	}
}

func TestBuildPodUsesContextDirAndWholeConfigMap(t *testing.T) {
	ci := testCI()
	pod := buildPod(ci, testService(), ci.Spec.Revision, "ttl.sh/team/web:0123456789ab", "")

	build := pod.Spec.Containers[0]
	if !hasEnv(build.Env, "CONTEXT_DIR", "services/web") {
		t.Errorf("build env lacks CONTEXT_DIR: %+v", build.Env)
	}
	if strings.Contains(build.Command[2], "services/web") {
		t.Error("the context dir must reach the script only as an env var")
	}
	if !strings.Contains(build.Command[2], ".dockerignore") {
		t.Error("the build script must install a generated .dockerignore")
	}
	for _, v := range pod.Spec.Volumes {
		if v.Name == "dockerfile" && v.ConfigMap != nil && len(v.ConfigMap.Items) != 0 {
			t.Error("the Dockerfile ConfigMap must be mounted whole so .dockerignore is optional")
		}
	}
}

func TestDiscoveryRequestAndStatus(t *testing.T) {
	svc := testService()
	svc.Spec.Port = 9000
	svc.Spec.Build = &koptanv1.BuildSpec{ContextDir: "api", DockerfilePath: "api/Dockerfile", Language: "go"}
	req := discoveryRequest(svc, nil, "tok")
	if req.Port != 9000 || req.ContextDir != "api" || req.DockerfilePath != "api/Dockerfile" ||
		req.Language != "go" || req.Token != "tok" {
		t.Errorf("unexpected request %+v", req)
	}
	if got := discoveryRequest(testService(), nil, "").Port; got != defaultPort {
		t.Errorf("default port = %d", got)
	}

	recordDiscovery(svc, &service.Result{
		Source: service.SourceTemplate,
		Facts:  &languages.Facts{Language: "go", Version: "1.24", Entrypoint: "./cmd/api"},
	})
	if svc.Status.ServiceType != "go" || svc.Status.DockerfileSource != "template" ||
		svc.Status.Detected == nil || svc.Status.Detected.Entrypoint != "./cmd/api" {
		t.Errorf("unexpected status %+v", svc.Status)
	}
	recordDiscovery(svc, &service.Result{Source: service.SourceRepo})
	if svc.Status.ServiceType != service.LanguageDockerfile || svc.Status.Detected != nil {
		t.Errorf("repository Dockerfile without a stack: %+v", svc.Status)
	}
}

func TestValidateSourceRejectsEscapingPaths(t *testing.T) {
	svc := testService()
	svc.Spec.Build = &koptanv1.BuildSpec{ContextDir: "../etc"}
	if err := validateSource(svc); err == nil {
		t.Error("want an error for a context dir with ..")
	}
	svc.Spec.Build = &koptanv1.BuildSpec{DockerfilePath: "a/../../b"}
	if err := validateSource(svc); err == nil {
		t.Error("want an error for a Dockerfile path with ..")
	}
}

func hasEnv(env []corev1.EnvVar, name, value string) bool {
	for _, e := range env {
		if e.Name == name && e.Value == value {
			return true
		}
	}
	return false
}
