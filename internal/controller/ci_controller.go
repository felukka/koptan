// Package controller provides the CI (Continuous Integration) reconciler
// for the koptan operator. It builds container images from Service sources
// and creates CD CRDs on success.
package controller

import (
	"context"
	"fmt"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/ci"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ciFinalizer = "felukka.org/ci-cleanup"
)

// CIReconciler reconciles a CI object by building a container image
// from the referenced Service and creating a CD CRD on success.
type CIReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Builder ci.Builder
}

// NewCIReconciler creates a CIReconciler with the default BuildahBuilder.
func NewCIReconciler(c client.Client, s *runtime.Scheme) *CIReconciler {
	return &CIReconciler{
		Client:  c,
		Scheme:  s,
		Builder: &ci.BuildahBuilder{},
	}
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cds,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile implements the CI reconciliation loop.
// Lifecycle: Idle -> Building (runs builder) -> Succeeded (creates CD).
func (r *CIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var ciObj koptanv1.CI
	if err := r.Get(ctx, req.NamespacedName, &ciObj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// --- Deletion handling ---
	if !ciObj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&ciObj, ciFinalizer) {
			controllerutil.RemoveFinalizer(&ciObj, ciFinalizer)
			return ctrl.Result{}, r.Update(ctx, &ciObj)
		}
		return ctrl.Result{}, nil
	}

	// --- Add finalizer ---
	if !controllerutil.ContainsFinalizer(&ciObj, ciFinalizer) {
		controllerutil.AddFinalizer(&ciObj, ciFinalizer)
		if err := r.Update(ctx, &ciObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// --- Skip if already succeeded ---
	if ciObj.Status.Phase == koptanv1.CIPhaseSucceeded && ciObj.Status.Image != "" {
		return ctrl.Result{}, nil
	}

	// --- Skip if currently building ---
	if ciObj.Status.Phase == koptanv1.CIPhaseBuilding {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// --- Resolve the Service ---
	svc, err := r.resolveService(ctx, &ciObj)
	if err != nil {
		r.setCIFailed(ctx, &ciObj, "ServiceResolveFailed", fmt.Sprintf("resolve service: %v", err))
		return ctrl.Result{}, err
	}

	// --- Wait for Service to be Ready ---
	if svc.Status.Phase != koptanv1.ServicePhaseReady {
		msg := fmt.Sprintf("Waiting for Service %q to be ready (current phase: %s)",
			svc.Name, svc.Status.Phase)
		setCIPhase(&ciObj, koptanv1.CIPhaseIdle, msg)
		if err := r.patchStatus(ctx, &ciObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// --- Phase: Building ---
	log.Info("starting build", "service", svc.Name, "image", ciObj.Spec.Registry.Repo)
	setCIPhase(&ciObj, koptanv1.CIPhaseBuilding, "Build in progress")
	if err := r.patchStatus(ctx, &ciObj); err != nil {
		return ctrl.Result{}, err
	}

	// Run the builder.
	imageRef, err := r.Builder.Build(ctx, &ciObj, svc)
	if err != nil {
		r.setCIFailed(ctx, &ciObj, "BuildFailed", fmt.Sprintf("build: %v", err))
		return ctrl.Result{}, err
	}

	log.Info("build succeeded", "image", imageRef)
	ciObj.Status.Phase = koptanv1.CIPhaseSucceeded
	ciObj.Status.Image = imageRef
	ciObj.Status.Message = "build succeeded"
	now := metav1.Now()
	ciObj.Status.BuildTime = &now
	ciObj.Status.BuildCount++
	ciObj.Status.Revision = svc.Spec.Source.Revision

	meta.SetStatusCondition(&ciObj.Status.Conditions, metav1.Condition{
		Type:               "BuildSucceeded",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "BuildCompleted",
		Message:            fmt.Sprintf("Image %s", imageRef),
	})

	if err := r.patchStatus(ctx, &ciObj); err != nil {
		return ctrl.Result{}, err
	}

	// --- Create CD CRD ---
	if err := r.createCD(ctx, &ciObj, svc, imageRef); err != nil {
		log.Error(err, "failed to create CD CRD")
	}

	return ctrl.Result{}, nil
}

// resolveService fetches the Service referenced by this CI CRD.
func (r *CIReconciler) resolveService(ctx context.Context, ciObj *koptanv1.CI) (*koptanv1.Service, error) {
	svc := &koptanv1.Service{}
	key := types.NamespacedName{
		Name:      ciObj.Spec.Service.Name,
		Namespace: ciObj.Namespace,
	}
	if err := r.Get(ctx, key, svc); err != nil {
		return nil, fmt.Errorf("get service %q: %w", key, err)
	}
	return svc, nil
}

// createCD creates a CD CRD that deploys the built image.
func (r *CIReconciler) createCD(ctx context.Context, ciObj *koptanv1.CI, svc *koptanv1.Service, imageRef string) error {
	// Check if CD already exists for this service.
	cdName := fmt.Sprintf("%s-cd", svc.Name)
	var existingCD koptanv1.CD
	err := r.Get(ctx, types.NamespacedName{Name: cdName, Namespace: ciObj.Namespace}, &existingCD)
	if err == nil {
		// CD already exists — update the image reference if changed.
		if existingCD.Status.Image != imageRef {
			existingCD.Status.Image = imageRef
			existingCD.Status.Revision = svc.Spec.Source.Revision
			existingCD.Status.Message = "waiting for deployment"
			if existingCD.Status.Conditions == nil {
				existingCD.Status.Conditions = []metav1.Condition{}
			}
			meta.SetStatusCondition(&existingCD.Status.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionUnknown,
				LastTransitionTime: metav1.Now(),
				Reason:             "ImageUpdated",
				Message:            "New image available",
			})
			if err := r.Status().Update(ctx, &existingCD); err != nil {
				return fmt.Errorf("update CD status: %w", err)
			}
		}
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("check existing CD: %w", err)
	}

	cd := &koptanv1.CD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cdName,
			Namespace: ciObj.Namespace,
			Labels: map[string]string{
				"koptan.felukka.org/service": svc.Name,
				"koptan.felukka.org/ci":      ciObj.Name,
			},
		},
		Spec: koptanv1.CDSpec{
			CI: koptanv1.NamespacedObjectReference{
				Name: ciObj.Name,
			},
			Replicas: 1,
			Env:      svc.Spec.Env,
			Resources: &koptanv1.Resources{
				CPURequest:    ptrQ("100m"),
				CPULimit:      ptrQ("500m"),
				MemoryRequest: ptrQ("128Mi"),
				MemoryLimit:   ptrQ("256Mi"),
			},
		},
	}

	if err := controllerutil.SetControllerReference(ciObj, cd, r.Scheme); err != nil {
		return fmt.Errorf("set controller ref on CD: %w", err)
	}

	return r.Create(ctx, cd)
}

// patchStatus patches the CI status with the latest conditions.
func (r *CIReconciler) patchStatus(ctx context.Context, ciObj *koptanv1.CI) error {
	return r.Status().Update(ctx, ciObj)
}

// setCIPhase sets the CI phase and message.
func setCIPhase(ciObj *koptanv1.CI, phase koptanv1.CIPhase, message string) {
	ciObj.Status.Phase = phase
	ciObj.Status.Message = message
	if ciObj.Status.Conditions == nil {
		ciObj.Status.Conditions = []metav1.Condition{}
	}
	meta.SetStatusCondition(&ciObj.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             string(phase),
		Message:            message,
	})
}

// setCIFailed marks the CI as Failed.
func (r *CIReconciler) setCIFailed(ctx context.Context, ciObj *koptanv1.CI, reason, msg string) {
	ciObj.Status.Phase = koptanv1.CIPhaseFailed
	ciObj.Status.Message = msg
	if ciObj.Status.Conditions == nil {
		ciObj.Status.Conditions = []metav1.Condition{}
	}
	meta.SetStatusCondition(&ciObj.Status.Conditions, metav1.Condition{
		Type:               "BuildSucceeded",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	})
	_ = r.patchStatus(ctx, ciObj)
}

// ptrQ is a helper to create a pointer to a resource.Quantity.
func ptrQ(s string) *resource.Quantity {
	q := resource.MustParse(s)
	return &q
}

// SetupWithManager sets up the controller with the Manager.
func (r *CIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.CI{}).
		Owns(&koptanv1.CD{}).
		Named("ci").
		Complete(r)
}
