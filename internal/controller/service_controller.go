// Package controller provides the Service reconciler for the koptan operator.
// It discovers source languages, generates Dockerfiles, and manages the
// CI/CD pipeline lifecycle. When a Ready Service detects a new git commit,
// it re-triggers the full pipeline (discover → build → deploy).
package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/service"
	"github.com/felukka/koptan/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// serviceFinalizer is the finalizer applied to Service resources.
	serviceFinalizer = "felukka.org/service-cleanup"
	// pushPollInterval is how often to check for new commits on Ready services.
	pushPollInterval = 1 * time.Minute
)

// ServiceReconciler reconciles a Service object by discovering the source
// language, generating a Dockerfile if needed, and creating a CI CRD to
// trigger the build pipeline. On Ready services it polls for new git commits.
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cds,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile implements the Service reconciliation loop.
// Lifecycle: Pending -> Discovering -> Building (creates CI) -> Ready.
// On Ready: polls git for new commits and re-triggers the pipeline.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var svc koptanv1.Service
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// --- Deletion handling ---
	if !svc.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&svc, serviceFinalizer) {
			controllerutil.RemoveFinalizer(&svc, serviceFinalizer)
			return ctrl.Result{}, r.Update(ctx, &svc)
		}
		return ctrl.Result{}, nil
	}

	// --- Add finalizer ---
	if !controllerutil.ContainsFinalizer(&svc, serviceFinalizer) {
		controllerutil.AddFinalizer(&svc, serviceFinalizer)
		if err := r.Update(ctx, &svc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// --- Polling phase: check for new git commits on Ready services ---
	if svc.Status.Phase == koptanv1.ServicePhaseReady {
		// Update CDRef by finding owned CD resources.
		r.updateCDRef(ctx, &svc)
		return r.reconcileReady(ctx, &svc)
	}

	// --- Phase 1: Discovering ---
	if svc.Status.Phase != koptanv1.ServicePhaseDiscovering {
		setPhase(ctx, &svc, koptanv1.ServicePhaseDiscovering, "Starting discovery")
		if err := r.patchStatus(ctx, &svc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Run the discovery engine.
	log.Info("running discovery engine")
	engine := service.NewEngine()

	// Resolve git token from the secret if provided.
	if svc.Spec.Source.SecretRef != nil {
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Name:      svc.Spec.Source.SecretRef.Name,
			Namespace: req.Namespace,
		}
		if err := r.Get(ctx, secretKey, secret); err == nil {
			if key := svc.Spec.Source.SecretRef.Key; key != "" {
				os.Setenv("KOPTAN_GIT_TOKEN", string(secret.Data[key]))
			}
		}
	}
	ctx = logf.IntoContext(ctx, log)

	result, err := engine.Discover(ctx, &svc)
	if err != nil {
		r.setFailed(ctx, &svc, "DiscoveryFailed", fmt.Sprintf("discover: %v", err))
		return ctrl.Result{}, err
	}

	log.Info("discovery complete", "language", result.Language, "hasDockerfile", result.HasDockerfile)

	// Update status with discovered language.
	svc.Status.ServiceType = result.Language
	svc.Status.Error = ""

	// --- Phase 2: Building — create the CI CRD ---
	if err := r.createCI(ctx, &svc, result.Language); err != nil {
		r.setFailed(ctx, &svc, "CIInitFailed", fmt.Sprintf("create CI: %v", err))
		return ctrl.Result{}, err
	}

	setPhase(ctx, &svc, koptanv1.ServicePhaseBuilding,
		fmt.Sprintf("CI CRD created for %s", result.Language))
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:               "DockerfileGenerated",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "Generated",
		Message: fmt.Sprintf("Dockerfile %s for %s",
			map[bool]string{true: "detected", false: "generated"}[result.HasDockerfile],
			result.Language),
	})

	// Set observed generation.
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:               "Observed",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "ObservedGeneration",
		ObservedGeneration: svc.GetGeneration(),
	})

	if err := r.patchStatus(ctx, &svc); err != nil {
		return ctrl.Result{}, err
	}

	setPhase(ctx, &svc, koptanv1.ServicePhaseReady, "Discovery complete, CI created")
	if err := r.patchStatus(ctx, &svc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileReady checks the git repo for new commits on a Ready Service.
// If a new commit is detected, it re-triggers the full pipeline.
func (r *ServiceReconciler) reconcileReady(ctx context.Context, svc *koptanv1.Service) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Resolve git token.
	token := ""
	if svc.Spec.Source.SecretRef != nil {
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Name:      svc.Spec.Source.SecretRef.Name,
			Namespace: svc.Namespace,
		}
		if err := r.Get(ctx, secretKey, secret); err == nil {
			if key := svc.Spec.Source.SecretRef.Key; key != "" {
				token = string(secret.Data[key])
			}
		}
	}

	// Check for new commits.
	revision := svc.Spec.Source.Revision
	if revision == "" {
		revision = "main"
	}

	changed, newRevision, err := utils.HasRevisionChanged(ctx, svc.Spec.Source.Repo, revision, svc.Status.LatestRevision, token)
	if err != nil {
		log.Error(err, "failed to check for new commits, will retry")
		return ctrl.Result{RequeueAfter: pushPollInterval}, nil
	}

	if !changed {
		// No new commits — recheck after the poll interval.
		return ctrl.Result{RequeueAfter: pushPollInterval}, nil
	}

	// --- New commit detected! Re-trigger the pipeline ---
	log.Info("new commit detected, re-triggering pipeline",
		"previousRevision", svc.Status.LatestRevision,
		"newRevision", newRevision.SHA)

	// Update revision tracking fields.
	now := metav1.Now()
	svc.Status.LatestRevision = newRevision.SHA
	svc.Status.LastPushDetected = &now

	// Transition to Discovering to re-run the full pipeline.
	setPhase(ctx, svc, koptanv1.ServicePhaseDiscovering,
		fmt.Sprintf("New commit %s detected, re-discovering", newRevision.SHA[:12]))
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:               "CommitDetected",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "NewCommit",
		Message: fmt.Sprintf("New commit %s detected on %s — pipeline re-triggered",
			newRevision.SHA[:12], svc.Spec.Source.Repo),
	})

	if err := r.patchStatus(ctx, svc); err != nil {
		return ctrl.Result{}, err
	}

	// Delete the old CI so a fresh one gets created during discovery.
	oldCIName := svc.Status.CIRef
	if oldCIName != "" {
		oldCI := &koptanv1.CI{}
		if err := r.Get(ctx, types.NamespacedName{Name: oldCIName, Namespace: svc.Namespace}, oldCI); err == nil {
			if delErr := r.Delete(ctx, oldCI); delErr != nil {
				if ignored := client.IgnoreNotFound(delErr); ignored != nil {
					log.Error(ignored, "failed to delete old CI, will retry")
				}
			}
		}
	}

	// Delete the old CD so it gets recreated.
	oldCDName := svc.Status.CDRef
	if oldCDName != "" {
		oldCD := &koptanv1.CD{}
		if err := r.Get(ctx, types.NamespacedName{Name: oldCDName, Namespace: svc.Namespace}, oldCD); err == nil {
			if delErr := r.Delete(ctx, oldCD); delErr != nil {
				if ignored := client.IgnoreNotFound(delErr); ignored != nil {
					log.Error(ignored, "failed to delete old CD, will retry")
				}
			}
		}
	}

	// Requeue immediately to start the discovery cycle.
	return ctrl.Result{Requeue: true}, nil
}

// createCI creates a CI CRD that references this Service, triggering the build pipeline.
func (r *ServiceReconciler) createCI(ctx context.Context, svc *koptanv1.Service, language string) error {
	// Check if a CI CRD already exists for this service.
	var ciList koptanv1.CIList
	if err := r.List(ctx, &ciList, client.InNamespace(svc.Namespace)); err != nil {
		return err
	}
	for _, ci := range ciList.Items {
		if ci.Spec.Service.Name == svc.Name {
			// CI already exists — nothing to do.
			svc.Status.CIRef = ci.Name
			return nil
		}
	}

	ci := &koptanv1.CI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ci", svc.Name),
			Namespace: svc.Namespace,
			Labels: map[string]string{
				"koptan.felukka.org/service":  svc.Name,
				"koptan.felukka.org/language": language,
			},
		},
		Spec: koptanv1.CISpec{
			Service: koptanv1.NamespacedObjectReference{
				Name: svc.Name,
			},
			Registry: koptanv1.RegistrySpec{
				Registry: "docker.io",
				Repo:     fmt.Sprintf("%s/%s", "felukka", svc.Name),
			},
		},
	}

	if err := controllerutil.SetControllerReference(svc, ci, r.Scheme); err != nil {
		return fmt.Errorf("set controller ref on CI: %w", err)
	}

	if err := r.Create(ctx, ci); err != nil {
		return err
	}

	svc.Status.CIRef = ci.Name
	return nil
}

// updateCDRef finds the CD resource owned by this Service and updates the CDRef.
func (r *ServiceReconciler) updateCDRef(ctx context.Context, svc *koptanv1.Service) {
	var cdList koptanv1.CDList
	if err := r.List(ctx, &cdList, client.InNamespace(svc.Namespace)); err != nil {
		return
	}
	for _, cd := range cdList.Items {
		for _, ref := range cd.OwnerReferences {
			if ref.UID == svc.UID {
				svc.Status.CDRef = cd.Name
				return
			}
		}
	}
	// No owned CD found — clear the reference.
	svc.Status.CDRef = ""
}

// patchStatus patches the Service status with the latest conditions.
func (r *ServiceReconciler) patchStatus(ctx context.Context, svc *koptanv1.Service) error {
	return r.Status().Update(ctx, svc)
}

// setPhase sets the phase and message on the Service status.
func setPhase(ctx context.Context, svc *koptanv1.Service, phase koptanv1.ServicePhase, message string) {
	svc.Status.Phase = phase
	svc.Status.Message = message
	if svc.Status.Conditions == nil {
		svc.Status.Conditions = []metav1.Condition{}
	}
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             string(phase),
		Message:            message,
	})
}

// setFailed marks the Service as Failed with the given reason and message.
func (r *ServiceReconciler) setFailed(ctx context.Context, svc *koptanv1.Service, reason, msg string) {
	svc.Status.Phase = koptanv1.ServicePhaseFailed
	svc.Status.Error = msg
	if svc.Status.Conditions == nil {
		svc.Status.Conditions = []metav1.Condition{}
	}
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	})
	_ = r.patchStatus(ctx, svc)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.Service{}).
		Owns(&koptanv1.CI{}).
		Owns(&koptanv1.CD{}).
		Named("service").
		Complete(r)
}
