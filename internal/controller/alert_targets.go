package controller

import (
	"context"
	"fmt"
	"sort"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/alerting"
	"github.com/felukka/koptan/internal/notify"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// targets returns the Services the Alert selects, by name and by label.
func (r *AlertReconciler) targets(ctx context.Context, alert *koptanv1.Alert) ([]koptanv1.Service, error) {
	byName := map[string]koptanv1.Service{}
	if ref := alert.Spec.ServiceRef; ref != nil {
		var svc koptanv1.Service
		err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: alert.Namespace}, &svc)
		switch {
		case err == nil:
			byName[svc.Name] = svc
		case !apierrors.IsNotFound(err):
			return nil, err
		}
	}
	if alert.Spec.Selector != nil {
		sel, err := metav1.LabelSelectorAsSelector(alert.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("spec.selector: %w", err)
		}
		if !sel.Empty() {
			var list koptanv1.ServiceList
			if err := r.List(ctx, &list, client.InNamespace(alert.Namespace),
				client.MatchingLabelsSelector{Selector: sel}); err != nil {
				return nil, err
			}
			for _, svc := range list.Items {
				byName[svc.Name] = svc
			}
		}
	}
	out := make([]koptanv1.Service, 0, len(byName))
	for _, svc := range byName {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// snapshot reads the CI and CD the operator made for a Service.
func (r *AlertReconciler) snapshot(ctx context.Context, svc *koptanv1.Service) (alerting.Snapshot, error) {
	snap := alerting.Snapshot{Service: svc}
	byService := client.MatchingLabels{labelService: svc.Name}
	var cis koptanv1.CIList
	if err := r.List(ctx, &cis, client.InNamespace(svc.Namespace), byService); err != nil {
		return snap, err
	}
	for i := range cis.Items {
		if cis.Items[i].Spec.Service.Name == svc.Name {
			snap.CI = &cis.Items[i]
			break
		}
	}
	var cds koptanv1.CDList
	if err := r.List(ctx, &cds, client.InNamespace(svc.Namespace), byService); err != nil {
		return snap, err
	}
	if len(cds.Items) > 0 {
		snap.CD = &cds.Items[0]
	}
	return snap, nil
}

// notifiers builds one Notifier per channel from the referenced Secrets.
func (r *AlertReconciler) notifiers(ctx context.Context, alert *koptanv1.Alert) (map[string]notify.Notifier, error) {
	out := make(map[string]notify.Notifier, len(alert.Spec.Channels))
	for _, ch := range alert.Spec.Channels {
		url, err := r.secretValue(ctx, alert.Namespace, ch.URLSecretRef)
		if err != nil {
			return nil, fmt.Errorf("channel %s: %w", ch.Name, err)
		}
		cfg := notify.Config{Type: string(ch.Type), URL: url}
		if ch.SigningSecretRef != nil {
			if cfg.SigningKey, err = r.secretValue(ctx, alert.Namespace, *ch.SigningSecretRef); err != nil {
				return nil, fmt.Errorf("channel %s signing key: %w", ch.Name, err)
			}
		}
		n, err := notify.New(cfg, r.HTTPClient)
		if err != nil {
			return nil, fmt.Errorf("channel %s: %w", ch.Name, err)
		}
		out[ch.Name] = n
	}
	return out, nil
}

func (r *AlertReconciler) secretValue(ctx context.Context, namespace string, ref corev1.SecretKeySelector) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, &secret); err != nil {
		return "", fmt.Errorf("read secret %q: %w", ref.Name, err)
	}
	v, ok := secret.Data[ref.Key]
	if !ok || len(v) == 0 {
		return "", fmt.Errorf("secret %q has no key %q", ref.Name, ref.Key)
	}
	return string(v), nil
}
