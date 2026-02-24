package plugins

import (
	"crypto/sha256"
	"fmt"

	koptanv1 "github.com/felukka/koptan/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ignoreWrapper runs the original command ("$0" "$@") and turns a failure
// into a logged success, for failurePolicy Ignore.
const ignoreWrapper = `"$0" "$@" || echo "koptan: plugin $KOPTAN_PLUGIN failed with exit $?; ignored (failurePolicy: Ignore)"`

// Steps renders the plugins, in the given order, as build Pod init
// containers.
func Steps(list []koptanv1.CIPlugin, env BuildEnv) ([]corev1.Container, error) {
	out := make([]corev1.Container, 0, len(list))
	for i := range list {
		c, err := Step(&list[i], env)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Step renders one plugin as an init container.
func Step(p *koptanv1.CIPlugin, env BuildEnv) (corev1.Container, error) {
	if err := Validate(p); err != nil {
		return corev1.Container{}, fmt.Errorf("plugin %s: %w", p.Name, err)
	}
	r, _ := rendererFor(p)
	c, err := r.Container(p, env)
	if err != nil {
		return corev1.Container{}, fmt.Errorf("plugin %s: %w", p.Name, err)
	}
	c.Name = ContainerName(p.Name)
	c.Env = append(commonEnv(p, env), c.Env...)
	c.VolumeMounts = append(c.VolumeMounts,
		corev1.VolumeMount{Name: WorkspaceVolume, MountPath: WorkspacePath},
		corev1.VolumeMount{Name: ReportsVolume, MountPath: ReportsPath},
	)
	if c.WorkingDir == "" {
		c.WorkingDir = env.SourceDir()
	}
	c.Resources = r.Resources()
	if p.Spec.Resources != nil {
		c.Resources = *p.Spec.Resources.DeepCopy()
	}
	c.TerminationMessagePolicy = corev1.TerminationMessageFallbackToLogsOnError
	if p.Spec.FailurePolicy == koptanv1.FailurePolicyIgnore {
		c.Args = append(append([]string{}, c.Command...), c.Args...)
		c.Command = []string{"sh", "-c", ignoreWrapper}
	}
	return c, nil
}

// ContainerName is the init container name of a plugin; names too long
// for a container are shortened with a hash so they stay unique.
func ContainerName(plugin string) string {
	name := containerPrefix + plugin
	if len(name) <= 63 {
		return name
	}
	sum := sha256.Sum256([]byte(plugin))
	return fmt.Sprintf("%s%s-%x", containerPrefix, plugin[:47], sum[:4])
}

// commonEnv tells every step what it is building. Repository, revision and
// paths only ever reach scripts as environment variables.
func commonEnv(p *koptanv1.CIPlugin, env BuildEnv) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "KOPTAN_PLUGIN", Value: p.Name},
		{Name: "KOPTAN_SERVICE", Value: env.Service},
		{Name: "KOPTAN_NAMESPACE", Value: env.Namespace},
		{Name: "KOPTAN_REPO", Value: env.Repo},
		{Name: "KOPTAN_REVISION", Value: env.Revision},
		{Name: "KOPTAN_LANGUAGE", Value: env.Language},
		{Name: "WORKSPACE", Value: WorkspacePath},
		{Name: "CONTEXT_DIR", Value: env.ContextDir},
		{Name: "KOPTAN_SOURCE_DIR", Value: env.SourceDir()},
		{Name: "KOPTAN_REPORTS_DIR", Value: ReportsPath},
	}
}

func requirements(cpuReq, memReq, cpuLim, memLim string) corev1.ResourceRequirements {
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

func secretEnv(name string, ref corev1.SecretKeySelector) corev1.EnvVar {
	return corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: ref.DeepCopy()}}
}
