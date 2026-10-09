package controller

import (
	"context"
	"fmt"
	"strings"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/plugins"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// maxPluginMessage bounds a plugin's message in the CI status.
const maxPluginMessage = 500

// pluginSteps renders the CI's plugins as build Pod init containers.
func (r *CIReconciler) pluginSteps(ctx context.Context, ci *koptanv1.CI, svc *koptanv1.Service,
	sha string) ([]corev1.Container, error) {
	if len(ci.Spec.Plugins) == 0 {
		return nil, nil
	}
	list := make([]koptanv1.CIPlugin, 0, len(ci.Spec.Plugins))
	for _, ref := range ci.Spec.Plugins {
		var p koptanv1.CIPlugin
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ci.Namespace}, &p); err != nil {
			return nil, fmt.Errorf("CIPlugin %s: %w", ref.Name, err)
		}
		list = append(list, p)
	}
	return plugins.Steps(list, plugins.BuildEnv{
		Service:    svc.Name,
		Namespace:  ci.Namespace,
		Repo:       svc.Spec.Source.Repo,
		Revision:   sha,
		Language:   svc.Status.ServiceType,
		ContextDir: ci.Spec.ContextDir,
	})
}

// pendingPluginResults lists every plugin of a build that just started.
func pendingPluginResults(ci *koptanv1.CI) []koptanv1.PluginResult {
	if len(ci.Spec.Plugins) == 0 {
		return nil
	}
	out := make([]koptanv1.PluginResult, 0, len(ci.Spec.Plugins))
	for _, ref := range ci.Spec.Plugins {
		out = append(out, koptanv1.PluginResult{Name: ref.Name, Phase: koptanv1.PluginPhasePending})
	}
	return out
}

// pluginResults reads each plugin step's state from the build Pod.
func pluginResults(ci *koptanv1.CI, pod *corev1.Pod) []koptanv1.PluginResult {
	if len(ci.Spec.Plugins) == 0 {
		return nil
	}
	statuses := map[string]corev1.ContainerStatus{}
	for _, s := range pod.Status.InitContainerStatuses {
		statuses[s.Name] = s
	}
	out := make([]koptanv1.PluginResult, 0, len(ci.Spec.Plugins))
	for _, ref := range ci.Spec.Plugins {
		result := koptanv1.PluginResult{Name: ref.Name, Phase: koptanv1.PluginPhasePending}
		s, ok := statuses[plugins.ContainerName(ref.Name)]
		switch {
		case !ok:
		case s.State.Terminated != nil && s.State.Terminated.ExitCode == 0:
			result.Phase = koptanv1.PluginPhaseSucceeded
			result.Message = tail(s.State.Terminated.Message, maxPluginMessage)
		case s.State.Terminated != nil:
			t := s.State.Terminated
			result.Phase = koptanv1.PluginPhaseFailed
			result.Message = fmt.Sprintf("exit %d: %s", t.ExitCode, tail(firstNonEmpty(t.Message, t.Reason), maxPluginMessage))
		case s.State.Running != nil:
			result.Phase = koptanv1.PluginPhaseRunning
		case s.State.Waiting != nil && s.State.Waiting.Reason != "PodInitializing":
			result.Message = s.State.Waiting.Reason
		}
		out = append(out, result)
	}
	return out
}

// tail keeps the last n bytes of s, where tools print their summary.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
