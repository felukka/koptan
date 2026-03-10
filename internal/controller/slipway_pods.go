package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SlipwayReconciler) fetchBuildPod(
	ctx context.Context,
	namespace string,
	jobName string,
) (*corev1.Pod, error) {
	var podList corev1.PodList
	err := r.List(ctx, &podList, client.InNamespace(namespace), client.MatchingLabels{
		"job-name": jobName,
	})
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, nil
	}
	return &podList.Items[0], nil
}

func podStatusMessage(pod *corev1.Pod) string {
	if pod == nil {
		return "waiting for pod to be scheduled"
	}

	if pod.DeletionTimestamp != nil {
		return "pod is terminating"
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		return pendingMessage(pod)
	case corev1.PodRunning:
		return runningMessage(pod)
	case corev1.PodFailed:
		return failedMessage(pod)
	case corev1.PodSucceeded:
		return "build completed"
	default:
		return fmt.Sprintf("pod phase: %s", pod.Status.Phase)
	}
}

func pendingMessage(pod *corev1.Pod) string {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			return fmt.Sprintf("unschedulable: %s", cond.Message)
		}
	}

	if msg := firstWaitingReason(pod.Status.InitContainerStatuses); msg != "" {
		return msg
	}

	if msg := firstWaitingReason(pod.Status.ContainerStatuses); msg != "" {
		return msg
	}

	return "pod is pending"
}

func runningMessage(pod *corev1.Pod) string {
	for i := range pod.Status.InitContainerStatuses {
		cs := &pod.Status.InitContainerStatuses[i]

		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return fmt.Sprintf(
				"init container %q failed (exit %d): %s",
				cs.Name, cs.State.Terminated.ExitCode, cs.State.Terminated.Reason,
			)
		}

		if cs.State.Running != nil {
			return fmt.Sprintf("running init step: %s", cs.Name)
		}

		if cs.State.Waiting != nil {
			return waitingMessage(cs.Name, cs.State.Waiting)
		}
	}

	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]

		if cs.State.Running != nil {
			return "building and pushing image"
		}

		if cs.State.Waiting != nil {
			return waitingMessage(cs.Name, cs.State.Waiting)
		}
	}

	return "build running"
}

func failedMessage(pod *corev1.Pod) string {
	all := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for i := range all {
		cs := &all[i]
		if t := cs.State.Terminated; t != nil && t.ExitCode != 0 {
			msg := fmt.Sprintf("container %q failed (exit %d)", cs.Name, t.ExitCode)
			if t.Reason != "" {
				msg += fmt.Sprintf(": %s", t.Reason)
			}
			if t.Message != "" {
				msg += fmt.Sprintf(" — %s", t.Message)
			}
			return msg
		}
	}

	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	return "pod failed"
}

func firstWaitingReason(statuses []corev1.ContainerStatus) string {
	for i := range statuses {
		if w := statuses[i].State.Waiting; w != nil {
			return waitingMessage(statuses[i].Name, w)
		}
	}
	return ""
}

func waitingMessage(name string, w *corev1.ContainerStateWaiting) string {
	switch w.Reason {
	case "ImagePullBackOff", "ErrImagePull":
		return fmt.Sprintf("image pull failed for %q: %s", name, w.Message)
	case "CrashLoopBackOff":
		return fmt.Sprintf("container %q is crash-looping: %s", name, w.Message)
	case "CreateContainerConfigError":
		return fmt.Sprintf("config error for %q: %s", name, w.Message)
	case "ContainerCreating", "PodInitializing":
		return fmt.Sprintf("pulling images for %q", name)
	default:
		if w.Message != "" {
			return fmt.Sprintf("waiting (%s): %s", w.Reason, w.Message)
		}
		if w.Reason != "" {
			return fmt.Sprintf("waiting: %s", w.Reason)
		}
		return fmt.Sprintf("container %q is waiting", name)
	}
}
