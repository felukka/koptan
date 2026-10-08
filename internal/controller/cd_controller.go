// Package controller provides the CD (Continuous Deployment) reconciler
// for the koptan operator. It creates Kubernetes Deployments and Services
// from CD CRDs, with proper OwnerReferences for automatic cleanup.
package controller

import (
	"context"
	"fmt"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	cdFinalizer = "felukka.org/cd-cleanup"
)

// CDReconciler reconciles a CD object by creating and managing Kubernetes
// Deployment and Service child resources.
type CDReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cds/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile implements the CD reconciliation loop.
// Lifecycle: Waiting -> Deploying -> Running.
func (r *CDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var cd koptanv1.CD
	if err := r.Get(ctx, req.NamespacedName, &cd); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// --- Deletion handling ---
	if !cd.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&cd, cdFinalizer) {
			controllerutil.RemoveFinalizer(&cd, cdFinalizer)
			return ctrl.Result{}, r.Update(ctx, &cd)
		}
		return ctrl.Result{}, nil
	}

	// --- Add finalizer ---
	if !controllerutil.ContainsFinalizer(&cd, cdFinalizer) {
		controllerutil.AddFinalizer(&cd, cdFinalizer)
		if err := r.Update(ctx, &cd); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// --- Skip if already Running ---
	if cd.Status.Phase == koptanv1.CDPhaseRunning {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// --- Phase: Deploying ---
	cd.Status.Phase = koptanv1.CDPhaseDeploying
	cd.Status.Message = "Starting deployment"
	setCDPhase(&cd, koptanv1.CDPhaseDeploying, "Starting deployment")
	if err := r.patchStatus(ctx, &cd); err != nil {
		return ctrl.Result{}, err
	}

	// Resolve the CI to get the image.
	ci := &koptanv1.CI{}
	ciKey := types.NamespacedName{
		Name:      cd.Spec.CI.Name,
		Namespace: cd.Namespace,
	}
	if err := r.Get(ctx, ciKey, ci); err != nil {
		r.setCDFailed(ctx, &cd, "CIResolveFailed", fmt.Sprintf("resolve CI: %v", err))
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	if ci.Status.Phase != koptanv1.CIPhaseSucceeded || ci.Status.Image == "" {
		msg := fmt.Sprintf("Waiting for CI %q to succeed (current phase: %s)",
			ci.Name, ci.Status.Phase)
		r.setCDWaiting(ctx, &cd, msg)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	image := ci.Status.Image
	log.Info("deploying image", "image", image)

	// --- Reconcile Deployment ---
	if err := r.reconcileDeployment(ctx, &cd, image); err != nil {
		r.setCDFailed(ctx, &cd, "DeploymentFailed",
			fmt.Sprintf("create/update deployment: %v", err))
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// --- Reconcile Service ---
	if err := r.reconcileService(ctx, &cd); err != nil {
		r.setCDFailed(ctx, &cd, "ServiceFailed",
			fmt.Sprintf("create/update service: %v", err))
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	// --- Mark Running ---
	cd.Status.Phase = koptanv1.CDPhaseRunning
	cd.Status.Image = image
	cd.Status.Revision = cd.Spec.CI.Name
	cd.Status.Message = "deployment complete"
	cd.Status.Conditions = []metav1.Condition{{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "Deployed",
		Message:            fmt.Sprintf("Image %s deployed", image),
	}}

	if err := r.patchStatus(ctx, &cd); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("deployment complete", "image", image)
	return ctrl.Result{}, nil
}

// reconcileDeployment creates or updates the Kubernetes Deployment.
func (r *CDReconciler) reconcileDeployment(ctx context.Context, cd *koptanv1.CD, image string) error {
	log := logf.FromContext(ctx)

	replicas := cd.Spec.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	labels := map[string]string{
		"koptan.felukka.org/cd":        cd.Name,
		"koptan.felukka.org/component": "app",
	}

	envVars := make([]corev1.EnvVar, 0, len(cd.Spec.Env)+1)
	envVars = append(envVars, corev1.EnvVar{
		Name:  "CONTAINER_IMAGE",
		Value: image,
	})
	envVars = append(envVars, cd.Spec.Env...)

	container := corev1.Container{
		Name:  cd.Name,
		Image: image,
		Env:   envVars,
		Ports: []corev1.ContainerPort{{
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		}},
	}

	// Apply resource limits from the CRD spec.
	if cd.Spec.Resources != nil {
		if cd.Spec.Resources.CPURequest != nil {
			container.Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU: *cd.Spec.Resources.CPURequest,
			}
		}
		if cd.Spec.Resources.MemoryRequest != nil {
			if container.Resources.Requests == nil {
				container.Resources.Requests = corev1.ResourceList{}
			}
			container.Resources.Requests[corev1.ResourceMemory] = *cd.Spec.Resources.MemoryRequest
		}
		if cd.Spec.Resources.CPULimit != nil {
			container.Resources.Limits = corev1.ResourceList{
				corev1.ResourceCPU: *cd.Spec.Resources.CPULimit,
			}
		}
		if cd.Spec.Resources.MemoryLimit != nil {
			if container.Resources.Limits == nil {
				container.Resources.Limits = corev1.ResourceList{}
			}
			container.Resources.Limits[corev1.ResourceMemory] = *cd.Spec.Resources.MemoryLimit
		}
	}

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cd.Name,
			Namespace: cd.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(cd, desired, r.Scheme); err != nil {
		return fmt.Errorf("set controller ref on Deployment: %w", err)
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: cd.Name, Namespace: cd.Namespace}, &existing)
	if errors.IsNotFound(err) {
		log.Info("creating Deployment", "deployment", cd.Name)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("get deployment %s: %w", cd.Name, err)
	}

	// Update if specs differ.
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels
	log.Info("updating Deployment", "deployment", cd.Name)
	return r.Update(ctx, &existing)
}

// reconcileService creates or updates the Kubernetes headless Service.
func (r *CDReconciler) reconcileService(ctx context.Context, cd *koptanv1.CD) error {
	log := logf.FromContext(ctx)

	labels := map[string]string{
		"koptan.felukka.org/cd":        cd.Name,
		"koptan.felukka.org/component": "app",
	}

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cd.Name,
			Namespace: cd.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt(8080),
				Protocol:   corev1.ProtocolTCP,
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	if err := controllerutil.SetControllerReference(cd, desired, r.Scheme); err != nil {
		return fmt.Errorf("set controller ref on Service: %w", err)
	}

	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: cd.Name, Namespace: cd.Namespace}, &existing)
	if errors.IsNotFound(err) {
		log.Info("creating Service", "service", cd.Name)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("get service %s: %w", cd.Name, err)
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	log.Info("updating Service", "service", cd.Name)
	return r.Update(ctx, &existing)
}

// patchStatus patches the CD status with the latest conditions.
func (r *CDReconciler) patchStatus(ctx context.Context, cd *koptanv1.CD) error {
	return r.Status().Update(ctx, cd)
}

// setCDPhase sets the CD phase and message on the object (caller must persist).
func setCDPhase(cd *koptanv1.CD, phase koptanv1.CDPhase, message string) {
	cd.Status.Phase = phase
	cd.Status.Message = message
	cd.Status.Conditions = []metav1.Condition{{
		Type:               "Ready",
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             string(phase),
		Message:            message,
	}}
}

// setCDWaiting marks the CD as Waiting for CI.
func (r *CDReconciler) setCDWaiting(ctx context.Context, cd *koptanv1.CD, message string) {
	cd.Status.Phase = koptanv1.CDPhaseWaiting
	cd.Status.Message = message
	if cd.Status.Conditions == nil {
		cd.Status.Conditions = []metav1.Condition{}
	}
	meta.SetStatusCondition(&cd.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "WaitingForCI",
		Message:            message,
	})
	_ = r.patchStatus(ctx, cd)
}

// setCDFailed marks the CD as Failed.
func (r *CDReconciler) setCDFailed(ctx context.Context, cd *koptanv1.CD, reason, msg string) {
	cd.Status.Phase = koptanv1.CDPhaseFailed
	cd.Status.Message = msg
	if cd.Status.Conditions == nil {
		cd.Status.Conditions = []metav1.Condition{}
	}
	meta.SetStatusCondition(&cd.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	})
	_ = r.patchStatus(ctx, cd)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.CD{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("cd").
		Complete(r)
}
