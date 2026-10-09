package controller

import (
	"context"
	"sort"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/plugins"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// CIPluginReconciler reports whether a CIPlugin is valid (Accepted) and
// which Services run it. Attaching it to builds is the Service and CI
// controllers' job; a plugin owns nothing, so it needs no finalizer.
type CIPluginReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=ciplugins,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=ciplugins/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services,verbs=get;list;watch

func (r *CIPluginReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var p koptanv1.CIPlugin
	if err := r.Get(ctx, req.NamespacedName, &p); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	orig := p.DeepCopy()

	cond := metav1.Condition{Type: "Accepted", Status: metav1.ConditionTrue, Reason: "Valid",
		Message: "The plugin runs after checkout and before build/push", ObservedGeneration: p.Generation}
	if err := plugins.Validate(&p); err != nil {
		cond.Status, cond.Reason, cond.Message = metav1.ConditionFalse, "Invalid", err.Error()
	}
	meta.SetStatusCondition(&p.Status.Conditions, cond)

	var services koptanv1.ServiceList
	if err := r.List(ctx, &services, client.InNamespace(p.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	attached := []string{}
	for i := range services.Items {
		if plugins.AttachedTo(&p, &services.Items[i]) {
			attached = append(attached, services.Items[i].Name)
		}
	}
	sort.Strings(attached)
	p.Status.AttachedServices = attached
	p.Status.ObservedGeneration = p.Generation
	return ctrl.Result{}, r.Status().Patch(ctx, &p, client.MergeFrom(orig))
}

// pluginsForService enqueues the plugins in a Service's namespace, since a
// Service change can attach or detach any of them.
func (r *CIPluginReconciler) pluginsForService(ctx context.Context, o client.Object) []ctrl.Request {
	var list koptanv1.CIPluginList
	if err := r.List(ctx, &list, client.InNamespace(o.GetNamespace())); err != nil {
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(list.Items))
	for _, p := range list.Items {
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: p.Namespace, Name: p.Name}})
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *CIPluginReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.CIPlugin{}).
		Watches(&koptanv1.Service{}, handler.EnqueueRequestsFromMapFunc(r.pluginsForService)).
		Named("ciplugin").
		Complete(r)
}
