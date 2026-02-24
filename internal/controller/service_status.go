package controller

import (
	"context"
	"fmt"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// gitToken reads the token named by spec.source.secretRef, if any.
func (r *ServiceReconciler) gitToken(ctx context.Context, svc *koptanv1.Service) (string, error) {
	ref := svc.Spec.Source.SecretRef
	if ref == nil {
		return "", nil
	}
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: svc.Namespace}, &secret); err != nil {
		return "", fmt.Errorf("read git token secret %q: %w", ref.Name, err)
	}
	key := ref.Key
	if key == "" {
		key = "token"
	}
	token, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %q has no key %q", ref.Name, key)
	}
	return string(token), nil
}

// validateSource rejects specs that can never work, so they fail once
// instead of retrying.
func validateSource(svc *koptanv1.Service) error {
	if err := utils.ValidateGitURL(svc.Spec.Source.Repo); err != nil {
		return err
	}
	if err := utils.ValidateRevision(svc.Spec.Source.Revision); err != nil {
		return err
	}
	if b := svc.Spec.Build; b != nil {
		if err := utils.ValidateRelPath("spec.build.contextDir", b.ContextDir); err != nil {
			return err
		}
		return utils.ValidateRelPath("spec.build.dockerfilePath", b.DockerfilePath)
	}
	return nil
}

// servicePort is spec.port, or the default.
func servicePort(svc *koptanv1.Service) int32 {
	if svc.Spec.Port != 0 {
		return svc.Spec.Port
	}
	return defaultPort
}

// retryWait is how long a Failed Service still waits before retrying.
func retryWait(svc *koptanv1.Service) time.Duration {
	c := meta.FindStatusCondition(svc.Status.Conditions, "Ready")
	if c == nil {
		return 0
	}
	return time.Until(c.LastTransitionTime.Add(failedRetryInterval))
}

// patchStatus writes status changes as a merge patch, so concurrent spec or
// metadata updates do not make the write fail.
func (r *ServiceReconciler) patchStatus(ctx context.Context, orig, svc *koptanv1.Service) error {
	return r.Status().Patch(ctx, svc, client.MergeFrom(orig))
}

// setPhase sets the phase and message on the Service status.
func setPhase(svc *koptanv1.Service, phase koptanv1.ServicePhase, message string) {
	svc.Status.Phase = phase
	svc.Status.Message = message
	if phase != koptanv1.ServicePhaseReady {
		meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionUnknown,
			Reason:  string(phase),
			Message: message,
		})
	}
}

// markFailed marks the Service as Failed with the given reason and message.
func (r *ServiceReconciler) markFailed(svc *koptanv1.Service, reason, msg string) {
	svc.Status.Phase = koptanv1.ServicePhaseFailed
	svc.Status.Error = msg
	svc.Status.Message = msg
	svc.Status.ObservedGeneration = svc.Generation
	meta.RemoveStatusCondition(&svc.Status.Conditions, "Ready")
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: msg,
	})
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
