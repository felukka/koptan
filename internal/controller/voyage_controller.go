package controller

import (
	"context"
	"fmt"
	"time"

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

	koptan "github.com/felukka/koptan/api/v1alpha"
)

const voyageFinalizer = "felukka.org/voyage-cleanup"

type VoyageReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=voyages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=voyages/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=voyages/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=slipways,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *VoyageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var voyage koptan.Voyage
	if err := r.Get(ctx, req.NamespacedName, &voyage); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !voyage.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&voyage, voyageFinalizer) {
			controllerutil.RemoveFinalizer(&voyage, voyageFinalizer)
			return ctrl.Result{}, r.Update(ctx, &voyage)
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&voyage, voyageFinalizer) {
		controllerutil.AddFinalizer(&voyage, voyageFinalizer)
		if err := r.Update(ctx, &voyage); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	var sw koptan.Slipway
	if err := r.Get(ctx, types.NamespacedName{Name: voyage.Spec.SlipwayRef.Name, Namespace: voyage.Namespace}, &sw); err != nil {
		log.Error(err, "slipway not found", "slipway", voyage.Spec.SlipwayRef.Name)
		r.setVoyageStatus(ctx, &voyage, koptan.VoyagePhaseWaiting, "", "slipway not found")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	if sw.Status.Phase != koptan.SlipwayPhaseSucceeded || sw.Status.LatestImage == "" {
		log.Info("slipway not ready", "phase", sw.Status.Phase)
		r.setVoyageStatus(ctx, &voyage, koptan.VoyagePhaseWaiting, "", fmt.Sprintf("waiting for slipway (phase: %s)", sw.Status.Phase))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	image := sw.Status.LatestImage

	if err := r.reconcileDeployment(ctx, &voyage, image); err != nil {
		log.Error(err, "failed to reconcile deployment")
		r.setVoyageStatus(ctx, &voyage, koptan.VoyagePhaseFailed, image, fmt.Sprintf("deployment failed: %v", err))
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	if err := r.reconcileService(ctx, &voyage); err != nil {
		log.Error(err, "failed to reconcile service")
		r.setVoyageStatus(ctx, &voyage, koptan.VoyagePhaseFailed, image, fmt.Sprintf("service failed: %v", err))
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	r.setVoyageStatus(ctx, &voyage, koptan.VoyagePhaseRunning, image, "")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *VoyageReconciler) reconcileDeployment(ctx context.Context, voyage *koptan.Voyage, image string) error {
	labels := map[string]string{
		"felukka.org/voyage":    voyage.Name,
		"felukka.org/component": "app",
	}

	replicas := voyage.Spec.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	container := corev1.Container{
		Name:  voyage.Name,
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: voyage.Spec.Port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: voyage.Spec.Env,
	}

	if voyage.Spec.Resources != nil {
		container.Resources = voyage.Spec.Resources.ToK8s()
	}

	if hc := voyage.Spec.HealthCheck; hc != nil {
		port := hc.Port
		if port == 0 {
			port = voyage.Spec.Port
		}
		path := hc.Path
		if path == "" {
			path = "/healthz"
		}
		initialDelay := hc.InitialDelaySeconds
		if initialDelay == 0 {
			initialDelay = 30
		}
		period := hc.PeriodSeconds
		if period == 0 {
			period = 10
		}

		probe := &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: path,
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: initialDelay,
			PeriodSeconds:       period,
		}

		container.LivenessProbe = probe
		container.ReadinessProbe = probe.DeepCopy()
	}

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      voyage.Name,
			Namespace: voyage.Namespace,
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

	if err := controllerutil.SetControllerReference(voyage, desired, r.Scheme); err != nil {
		return err
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: voyage.Name, Namespace: voyage.Namespace}, &existing)

	if errors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	return r.Update(ctx, &existing)
}

func (r *VoyageReconciler) reconcileService(ctx context.Context, voyage *koptan.Voyage) error {
	labels := map[string]string{
		"felukka.org/voyage":    voyage.Name,
		"felukka.org/component": "app",
	}

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      voyage.Name,
			Namespace: voyage.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       voyage.Spec.Port,
					TargetPort: intstr.FromInt32(voyage.Spec.Port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(voyage, desired, r.Scheme); err != nil {
		return err
	}

	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: voyage.Name, Namespace: voyage.Namespace}, &existing)

	if errors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Ports = desired.Spec.Ports
	return r.Update(ctx, &existing)
}

func (r *VoyageReconciler) setVoyageStatus(ctx context.Context, voyage *koptan.Voyage, phase koptan.VoyagePhase, image, msg string) {
	voyage.Status.Phase = phase
	voyage.Status.DeployedImage = image

	if msg != "" {
		meta.SetStatusCondition(&voyage.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  string(phase),
			Message: msg,
		})
	} else if phase == koptan.VoyagePhaseRunning {
		meta.SetStatusCondition(&voyage.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "Running",
			Message: fmt.Sprintf("deployed image %s", image),
		})
	}

	_ = r.Status().Update(ctx, voyage)
}

func (r *VoyageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptan.Voyage{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("voyage").
		Complete(r)
}
