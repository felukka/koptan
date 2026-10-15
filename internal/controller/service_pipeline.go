package controller

import (
	"context"
	"fmt"

	koptanv1 "github.com/felukka/koptan/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Ready-condition reasons for a Service whose own steps worked but whose
// pipeline did not. Unlike the Service's own failures they are not retried
// by rediscovery: the CI or CD has to change.
const (
	reasonCIFailed = "CIFailed"
	reasonCDFailed = "CDFailed"
)

// pipelineFailure reports whether a Failed Service failed in its CI or CD
// rather than in its own steps (source, secret, revision, discovery).
func pipelineFailure(svc *koptanv1.Service) bool {
	c := meta.FindStatusCondition(svc.Status.Conditions, "Ready")
	return svc.Status.Phase == koptanv1.ServicePhaseFailed && c != nil &&
		(c.Reason == reasonCIFailed || c.Reason == reasonCDFailed)
}

// discovered reports whether discovery finished for the current spec, so
// the Service only polls git for new commits instead of rediscovering.
func discovered(svc *koptanv1.Service) bool {
	if svc.Status.ObservedGeneration != svc.Generation || svc.Status.CIRef == "" {
		return false
	}
	switch svc.Status.Phase {
	case koptanv1.ServicePhaseBuilding, koptanv1.ServicePhaseReady:
		return true
	}
	return pipelineFailure(svc)
}

// ensureOwnCI points the CI at what the last discovery found. Running it on
// every reconcile reverts hand edits to the fields the Service owns and
// recreates a deleted CI; an unchanged CI is not written.
func (r *ServiceReconciler) ensureOwnCI(ctx context.Context, svc *koptanv1.Service,
	pluginRefs []koptanv1.CIPluginRef) error {
	name, err := r.ensureCI(ctx, svc, ciTarget{
		language: svc.Status.ServiceType, sha: svc.Status.LatestRevision,
		dockerfileCM: svc.Status.DockerfileConfigMap, plugins: pluginRefs,
	})
	if err == nil {
		svc.Status.CIRef = name
	}
	return err
}

// setPipelinePhase derives the phase from the CI and CD: Building while the
// CI builds or the CD rolls out, Ready only once the CD runs the latest
// revision, Failed with their message when either fails.
func (r *ServiceReconciler) setPipelinePhase(ctx context.Context, svc *koptanv1.Service) error {
	rev := short(svc.Status.LatestRevision)
	var ci koptanv1.CI
	err := r.Get(ctx, types.NamespacedName{Name: svc.Status.CIRef, Namespace: svc.Namespace}, &ci)
	if apierrors.IsNotFound(err) {
		setPhase(svc, koptanv1.ServicePhaseBuilding, fmt.Sprintf("Waiting for CI %s", svc.Status.CIRef))
		return nil
	}
	if err != nil {
		return err
	}
	switch {
	case ci.Spec.Revision != svc.Status.LatestRevision:
		setPhase(svc, koptanv1.ServicePhaseBuilding, fmt.Sprintf("CI %s moving to %s", ci.Name, rev))
		return nil
	case ci.Status.Phase == koptanv1.CIPhaseFailed:
		setPipelineFailed(svc, reasonCIFailed, fmt.Sprintf("CI %s failed: %s", ci.Name, ci.Status.Message))
		return nil
	case ci.Status.Phase != koptanv1.CIPhaseSucceeded || ci.Status.Revision != svc.Status.LatestRevision:
		setPhase(svc, koptanv1.ServicePhaseBuilding, fmt.Sprintf("CI %s building %s", ci.Name, rev))
		return nil
	}

	var cd koptanv1.CD
	if svc.Status.CDRef == "" {
		setPhase(svc, koptanv1.ServicePhaseBuilding, fmt.Sprintf("Waiting for the CD of %s", rev))
		return nil
	}
	err = r.Get(ctx, types.NamespacedName{Name: svc.Status.CDRef, Namespace: svc.Namespace}, &cd)
	if apierrors.IsNotFound(err) {
		setPhase(svc, koptanv1.ServicePhaseBuilding, fmt.Sprintf("Waiting for CD %s", svc.Status.CDRef))
		return nil
	}
	if err != nil {
		return err
	}
	switch {
	case cd.Status.Phase == koptanv1.CDPhaseFailed:
		setPipelineFailed(svc, reasonCDFailed, fmt.Sprintf("CD %s failed: %s", cd.Name, cd.Status.Message))
	case cd.Status.Phase == koptanv1.CDPhaseRunning && cd.Status.Revision == svc.Status.LatestRevision:
		setPhase(svc, koptanv1.ServicePhaseReady, fmt.Sprintf("Running %s", rev))
		meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionTrue, Reason: "Running", Message: svc.Status.Message,
		})
	default:
		setPhase(svc, koptanv1.ServicePhaseBuilding, fmt.Sprintf("CD %s deploying %s", cd.Name, rev))
	}
	return nil
}

// setPipelineFailed marks the Service Failed for a CI or CD failure. Unlike
// markFailed it keeps status.error for the Service's own errors.
func setPipelineFailed(svc *koptanv1.Service, reason, msg string) {
	svc.Status.Phase = koptanv1.ServicePhaseFailed
	svc.Status.Message = msg
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason, Message: msg,
	})
}
