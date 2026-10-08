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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
// Reconcile deploys the CI's latest image. It runs on every change (CD
// spec, CI status, Deployment status), so replica, env, port and image
// changes are always applied.
func (r *CDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var cd koptanv1.CD
	if err := r.Get(ctx, req.NamespacedName, &cd); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cd.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&cd, cdFinalizer) {
			controllerutil.RemoveFinalizer(&cd, cdFinalizer)
			return ctrl.Result{}, r.Update(ctx, &cd)
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&cd, cdFinalizer) {
		controllerutil.AddFinalizer(&cd, cdFinalizer)
		if err := r.Update(ctx, &cd); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	orig := cd.DeepCopy()
	ci := &koptanv1.CI{}
	if err := r.Get(ctx, types.NamespacedName{Name: cd.Spec.CI.Name, Namespace: cd.Namespace}, ci); err != nil {
		setCDCondition(&cd, koptanv1.CDPhaseWaiting, metav1.ConditionFalse, "CIResolveFailed",
			fmt.Sprintf("resolve CI %q: %v", cd.Spec.CI.Name, err))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, r.patchStatus(ctx, orig, &cd)
	}

	// Deploy the last image that built successfully, even while a newer
	// build runs or after it failed.
	image := ci.Status.Image
	if image == "" {
		setCDCondition(&cd, koptanv1.CDPhaseWaiting, metav1.ConditionFalse, "WaitingForCI",
			fmt.Sprintf("Waiting for CI %q to build an image (phase: %s)", ci.Name, ci.Status.Phase))
		return ctrl.Result{}, r.patchStatus(ctx, orig, &cd)
	}

	deploy, err := r.reconcileDeployment(ctx, &cd, image)
	if err != nil {
		setCDCondition(&cd, koptanv1.CDPhaseFailed, metav1.ConditionFalse, "DeploymentFailed",
			fmt.Sprintf("create/update deployment: %v", err))
		_ = r.patchStatus(ctx, orig, &cd)
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, &cd); err != nil {
		setCDCondition(&cd, koptanv1.CDPhaseFailed, metav1.ConditionFalse, "ServiceFailed",
			fmt.Sprintf("create/update service: %v", err))
		_ = r.patchStatus(ctx, orig, &cd)
		return ctrl.Result{}, err
	}

	cd.Status.Image = image
	cd.Status.Revision = ci.Status.Revision
	cd.Status.AvailableReplicas = deploy.Status.AvailableReplicas
	want := int32(1)
	if deploy.Spec.Replicas != nil {
		want = *deploy.Spec.Replicas
	}
	rolledOut := deploy.Status.ObservedGeneration >= deploy.Generation &&
		deploy.Status.UpdatedReplicas >= want && deploy.Status.AvailableReplicas >= want
	switch {
	case rolledOut:
		setCDCondition(&cd, koptanv1.CDPhaseRunning, metav1.ConditionTrue, "Deployed",
			fmt.Sprintf("%d/%d replicas available, image %s", deploy.Status.AvailableReplicas, want, image))
	case deployFailed(deploy):
		setCDCondition(&cd, koptanv1.CDPhaseFailed, metav1.ConditionFalse, "ProgressDeadlineExceeded",
			fmt.Sprintf("deployment is not progressing: %d/%d replicas available", deploy.Status.AvailableReplicas, want))
	default:
		setCDCondition(&cd, koptanv1.CDPhaseDeploying, metav1.ConditionUnknown, "RollingOut",
			fmt.Sprintf("%d/%d replicas available, rolling out %s", deploy.Status.AvailableReplicas, want, image))
	}
	if err := r.patchStatus(ctx, orig, &cd); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("reconciled deployment", "image", image, "phase", cd.Status.Phase)
	return ctrl.Result{}, nil
}

func deployFailed(d *appsv1.Deployment) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Status == corev1.ConditionFalse &&
			c.Reason == "ProgressDeadlineExceeded" {
			return true
		}
	}
	return false
}

// reconcileDeployment creates or updates the Deployment and returns it with
// its current status.
func (r *CDReconciler) reconcileDeployment(ctx context.Context, cd *koptanv1.CD, image string) (*appsv1.Deployment, error) {
	replicas := cd.Spec.Replicas
	if replicas < 0 {
		replicas = 1
	}
	port := cd.Spec.Port
	if port == 0 {
		port = defaultPort
	}
	labels := map[string]string{
		"koptan.felukka.org/cd":        cd.Name,
		"koptan.felukka.org/component": "app",
	}

	env := []corev1.EnvVar{
		{Name: "CONTAINER_IMAGE", Value: image},
		{Name: "PORT", Value: fmt.Sprint(port)},
	}
	env = append(env, cd.Spec.Env...)

	container := corev1.Container{
		Name:  "app",
		Image: image,
		Env:   env,
		Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: port, Protocol: corev1.ProtocolTCP}},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(port)},
			},
			PeriodSeconds: 5,
		},
	}
	if res := cd.Spec.Resources; res != nil {
		container.Resources = corev1.ResourceRequirements{Requests: corev1.ResourceList{}, Limits: corev1.ResourceList{}}
		if res.CPURequest != nil {
			container.Resources.Requests[corev1.ResourceCPU] = *res.CPURequest
		}
		if res.MemoryRequest != nil {
			container.Resources.Requests[corev1.ResourceMemory] = *res.MemoryRequest
		}
		if res.CPULimit != nil {
			container.Resources.Limits[corev1.ResourceCPU] = *res.CPULimit
		}
		if res.MemoryLimit != nil {
			container.Resources.Limits[corev1.ResourceMemory] = *res.MemoryLimit
		}
	}

	podSpec := corev1.PodSpec{Containers: []corev1.Container{container}}
	if cd.Spec.ImagePullSecret != "" {
		podSpec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: cd.Spec.ImagePullSecret}}
	}

	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: cd.Name, Namespace: cd.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Labels = labels
		deploy.Spec.Replicas = &replicas
		if deploy.Spec.Selector == nil {
			deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		}
		deploy.Spec.Template.Labels = labels
		deploy.Spec.Template.Spec = podSpec
		return controllerutil.SetControllerReference(cd, deploy, r.Scheme)
	})
	return deploy, err
}

// reconcileService exposes the Deployment on port 80.
func (r *CDReconciler) reconcileService(ctx context.Context, cd *koptanv1.CD) error {
	port := cd.Spec.Port
	if port == 0 {
		port = defaultPort
	}
	labels := map[string]string{
		"koptan.felukka.org/cd":        cd.Name,
		"koptan.felukka.org/component": "app",
	}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: cd.Name, Namespace: cd.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = labels
		svc.Spec.Selector = labels
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.Ports = []corev1.ServicePort{{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		}}
		return controllerutil.SetControllerReference(cd, svc, r.Scheme)
	})
	return err
}

func (r *CDReconciler) patchStatus(ctx context.Context, orig, cd *koptanv1.CD) error {
	return r.Status().Patch(ctx, cd, client.MergeFrom(orig))
}

// setCDCondition sets the phase, message and Ready condition.
func setCDCondition(cd *koptanv1.CD, phase koptanv1.CDPhase, status metav1.ConditionStatus, reason, message string) {
	cd.Status.Phase = phase
	cd.Status.Message = message
	meta.SetStatusCondition(&cd.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: status, Reason: reason, Message: message,
	})
}

// SetupWithManager sets up the controller with the Manager. CI changes
// (a new image) re-trigger the CDs that reference the CI.
func (r *CDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.CD{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(&koptanv1.CI{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, o client.Object) []ctrl.Request {
				var cds koptanv1.CDList
				if err := mgr.GetClient().List(ctx, &cds, client.InNamespace(o.GetNamespace())); err != nil {
					return nil
				}
				var reqs []ctrl.Request
				for _, cd := range cds.Items {
					if cd.Spec.CI.Name == o.GetName() {
						reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
							Namespace: cd.Namespace, Name: cd.Name}})
					}
				}
				return reqs
			})).
		Named("cd").
		Complete(r)
}
