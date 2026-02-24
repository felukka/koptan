package controller

import (
	"context"
	"fmt"
	"strings"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/plugins"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// resolvePlugins returns the CIPlugins this Service's builds run, pinned
// at their generation. A plugin the Service names but that does not exist
// or is invalid is an error: a build must not silently skip a check.
func (r *ServiceReconciler) resolvePlugins(ctx context.Context, svc *koptanv1.Service) ([]koptanv1.CIPluginRef, error) {
	var list koptanv1.CIPluginList
	if err := r.List(ctx, &list, client.InNamespace(svc.Namespace)); err != nil {
		return nil, err
	}
	attached, missing := plugins.Resolve(svc, list.Items)
	if len(missing) > 0 {
		return nil, fmt.Errorf("CIPlugin %s not found in namespace %s", strings.Join(missing, ", "), svc.Namespace)
	}
	for i := range attached {
		if err := plugins.Validate(&attached[i]); err != nil {
			return nil, fmt.Errorf("CIPlugin %s is invalid: %w", attached[i].Name, err)
		}
	}
	return plugins.Refs(attached), nil
}

// servicesForPlugin enqueues every Service in the plugin's namespace: a
// plugin can attach through a Service's spec.plugins, its own targetRefs
// or a label selector, and a change can detach it too.
func (r *ServiceReconciler) servicesForPlugin(ctx context.Context, o client.Object) []ctrl.Request {
	var list koptanv1.ServiceList
	if err := r.List(ctx, &list, client.InNamespace(o.GetNamespace())); err != nil {
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(list.Items))
	for _, svc := range list.Items {
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: svc.Namespace, Name: svc.Name}})
	}
	return reqs
}
