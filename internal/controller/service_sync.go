package controller

import (
	"context"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// syncCD copies replicas, env and port from the Service to its CD and
// records the CD's name.
func (r *ServiceReconciler) syncCD(ctx context.Context, svc *koptanv1.Service) error {
	var cds koptanv1.CDList
	if err := r.List(ctx, &cds, client.InNamespace(svc.Namespace),
		client.MatchingLabels{labelService: svc.Name}); err != nil {
		return err
	}
	svc.Status.CDRef = ""
	for i := range cds.Items {
		cd := &cds.Items[i]
		if !cd.DeletionTimestamp.IsZero() {
			continue
		}
		svc.Status.CDRef = cd.Name
		patch := client.MergeFrom(cd.DeepCopy())
		applyServiceToCD(svc, cd)
		if err := r.Patch(ctx, cd, patch); err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}

// applyServiceToCD sets the CD fields that the Service owns.
func applyServiceToCD(svc *koptanv1.Service, cd *koptanv1.CD) {
	cd.Spec.Replicas = 1
	if svc.Spec.Replicas != nil {
		cd.Spec.Replicas = *svc.Spec.Replicas
	}
	cd.Spec.Env = svc.Spec.Env
	cd.Spec.Port = servicePort(svc)
}
