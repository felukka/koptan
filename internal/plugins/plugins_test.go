package plugins

import (
	"os/exec"
	"strings"
	"testing"

	koptanv1 "github.com/felukka/koptan/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func plugin(name string, order int32, spec koptanv1.CIPluginSpec) koptanv1.CIPlugin {
	spec.Order = order
	if spec.Type == "" {
		spec.Type = koptanv1.CIPluginCustom
		spec.Custom = &koptanv1.CustomStep{Image: "alpine", Command: []string{"true"}}
	}
	return koptanv1.CIPlugin{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "team", Generation: 3}, Spec: spec}
}

func svc() *koptanv1.Service {
	return &koptanv1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "team", Labels: map[string]string{"tier": "backend"}},
		Spec:       koptanv1.ServiceSpec{Plugins: []koptanv1.PluginRef{{Name: "lint"}, {Name: "sonar"}, {Name: "gone"}}},
	}
}

func TestResolveUnionsBothSidesInOrder(t *testing.T) {
	all := []koptanv1.CIPlugin{
		plugin("sonar", 50, koptanv1.CIPluginSpec{TargetRefs: []koptanv1.PluginTargetRef{{Name: "api"}}}),
		plugin("lint", 10, koptanv1.CIPluginSpec{}),
		plugin("codeql", 50, koptanv1.CIPluginSpec{Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"tier": "backend"}}}),
		plugin("other-svc", 1, koptanv1.CIPluginSpec{TargetRefs: []koptanv1.PluginTargetRef{{Name: "web"}}}),
		plugin("everything", 1, koptanv1.CIPluginSpec{Selector: &metav1.LabelSelector{}}),
	}
	other := plugin("lint", 0, koptanv1.CIPluginSpec{})
	other.Namespace = "elsewhere"
	all = append(all, other)

	attached, missing := Resolve(svc(), all)
	names := make([]string, 0, len(attached))
	for _, p := range attached {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "lint,codeql,sonar" {
		t.Errorf("attached = %s, want lint,codeql,sonar (order, then name; no duplicates)", got)
	}
	if len(missing) != 1 || missing[0] != "gone" {
		t.Errorf("missing = %v, want [gone]", missing)
	}
	if refs := Refs(attached); refs[0].Generation != 3 {
		t.Errorf("refs must pin the generation: %+v", refs)
	}
}

func TestStepWiring(t *testing.T) {
	env := BuildEnv{Service: "api", Namespace: "team", Repo: "https://x/y.git",
		Revision: "abc", Language: "node", ContextDir: "web"}
	p := plugin("lint", 1, koptanv1.CIPluginSpec{})
	c, err := Step(&p, env)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "plugin-lint" || c.WorkingDir != "/workspace/web" {
		t.Errorf("name %q, workdir %q", c.Name, c.WorkingDir)
	}
	mounts := map[string]string{}
	for _, m := range c.VolumeMounts {
		mounts[m.Name] = m.MountPath
	}
	if mounts[WorkspaceVolume] != WorkspacePath || mounts[ReportsVolume] != ReportsPath {
		t.Errorf("mounts = %v", mounts)
	}
	if !hasEnv(c.Env, "KOPTAN_REVISION", "abc") || !hasEnv(c.Env, "KOPTAN_SOURCE_DIR", "/workspace/web") {
		t.Errorf("env = %+v", c.Env)
	}
	if c.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
		t.Error("steps must report their log tail on failure")
	}
	long := ContainerName(strings.Repeat("a", 80))
	if len(long) > 63 || long == ContainerName(strings.Repeat("a", 81)) {
		t.Errorf("long names must be shortened uniquely: %q", long)
	}
}

func TestIgnoreWrapperSwallowsFailure(t *testing.T) {
	p := plugin("flaky", 1, koptanv1.CIPluginSpec{FailurePolicy: koptanv1.FailurePolicyIgnore,
		Type: koptanv1.CIPluginCustom, Custom: &koptanv1.CustomStep{Image: "alpine",
			Command: []string{"sh", "-c", "echo running; exit 7"}}})
	c, err := Step(&p, BuildEnv{})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(c.Command[0], append(c.Command[1:], c.Args...)...)
	cmd.Env = []string{"KOPTAN_PLUGIN=flaky", "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wrapped step must exit 0, got %v: %s", err, out)
	}
	if !strings.Contains(string(out), "running") || !strings.Contains(string(out), "exit 7; ignored") {
		t.Errorf("output = %q", out)
	}

	noCmd := plugin("bare", 1, koptanv1.CIPluginSpec{FailurePolicy: koptanv1.FailurePolicyIgnore,
		Type: koptanv1.CIPluginCustom, Custom: &koptanv1.CustomStep{Image: "alpine"}})
	if err := Validate(&noCmd); err == nil {
		t.Error("Ignore without a command cannot be wrapped and must be rejected")
	}
}

func TestSonarQube(t *testing.T) {
	no := false
	p := plugin("sonar", 1, koptanv1.CIPluginSpec{Type: koptanv1.CIPluginSonarQube,
		SonarQube: &koptanv1.SonarQubeConfig{
			HostURL:            "https://sonar.example.com",
			TokenSecretRef:     corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "sonar"}, Key: "token"},
			WaitForQualityGate: &no,
			ExtraArgs:          []string{"-Dsonar.exclusions=**/*_test.go"},
		}})
	c, err := Step(&p, BuildEnv{Service: "api", Namespace: "team", Revision: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(c.Args, " ")
	for _, want := range []string{"-Dsonar.projectKey=team_api", "-Dsonar.scm.revision=abc", "-Dsonar.exclusions"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q: %s", want, args)
		}
	}
	if strings.Contains(args, "qualitygate.wait") {
		t.Error("waitForQualityGate false must not wait")
	}
	if !hasSecretEnv(c.Env, "SONAR_TOKEN", "sonar") {
		t.Errorf("token must come from the Secret: %+v", c.Env)
	}

	bad := p.DeepCopy()
	bad.Spec.SonarQube.TokenSecretRef.Key = ""
	if err := Validate(bad); err == nil {
		t.Error("a token secret without a key must be rejected")
	}
}

func TestCodeQL(t *testing.T) {
	p := plugin("codeql", 1, koptanv1.CIPluginSpec{Type: koptanv1.CIPluginCodeQL,
		CodeQL: &koptanv1.CodeQLConfig{}})
	c, err := Step(&p, BuildEnv{Language: "node"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEnv(c.Env, "CODEQL_LANGUAGES", "javascript-typescript:javascript") ||
		!hasEnv(c.Env, "CODEQL_FAIL_ON", "error") || !hasEnv(c.Env, "CODEQL_SUITE", "code-scanning") {
		t.Errorf("env = %+v", c.Env)
	}
	if got := codeQLLanguages(nil, "php"); got != "" {
		t.Errorf("an unsupported stack must analyze nothing, got %q", got)
	}
	if got := codeQLLanguages([]string{"python", "java-kotlin"}, "go"); got != "python:python java-kotlin:java" {
		t.Errorf("configured languages win: %q", got)
	}
	bad := plugin("codeql", 1, koptanv1.CIPluginSpec{Type: koptanv1.CIPluginCodeQL,
		CodeQL: &koptanv1.CodeQLConfig{Languages: []string{"cobol; rm -rf /"}}})
	if err := Validate(&bad); err == nil {
		t.Error("unknown CodeQL languages must be rejected")
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

func hasSecretEnv(env []corev1.EnvVar, name, secret string) bool {
	for _, e := range env {
		if e.Name == name && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil &&
			e.ValueFrom.SecretKeyRef.Name == secret {
			return true
		}
	}
	return false
}
