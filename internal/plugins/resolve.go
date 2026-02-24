package plugins

import (
	"sort"

	koptanv1 "github.com/felukka/koptan/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Resolve returns the plugins attached to svc: the ones it lists in
// spec.plugins plus the ones whose targetRefs or selector match it, without
// duplicates, sorted by spec.order then name. missing lists names from
// spec.plugins that are not in all.
func Resolve(svc *koptanv1.Service, all []koptanv1.CIPlugin) (attached []koptanv1.CIPlugin, missing []string) {
	byName := make(map[string]*koptanv1.CIPlugin, len(all))
	for i := range all {
		if all[i].Namespace == svc.Namespace && all[i].DeletionTimestamp.IsZero() {
			byName[all[i].Name] = &all[i]
		}
	}
	seen := map[string]bool{}
	add := func(p *koptanv1.CIPlugin) {
		if !seen[p.Name] {
			seen[p.Name] = true
			attached = append(attached, *p)
		}
	}
	for _, ref := range svc.Spec.Plugins {
		if p, ok := byName[ref.Name]; ok {
			add(p)
		} else {
			missing = append(missing, ref.Name)
		}
	}
	for _, p := range byName {
		if Targets(p, svc) {
			add(p)
		}
	}
	sort.SliceStable(attached, func(i, j int) bool {
		if attached[i].Spec.Order != attached[j].Spec.Order {
			return attached[i].Spec.Order < attached[j].Spec.Order
		}
		return attached[i].Name < attached[j].Name
	})
	return attached, missing
}

// Targets reports whether the plugin attaches itself to svc through its
// targetRefs or selector.
func Targets(p *koptanv1.CIPlugin, svc *koptanv1.Service) bool {
	if p.Namespace != svc.Namespace {
		return false
	}
	for _, ref := range p.Spec.TargetRefs {
		if (ref.Kind == "" || ref.Kind == "Service") && ref.Name == svc.Name {
			return true
		}
	}
	if p.Spec.Selector == nil {
		return false
	}
	sel, err := metav1.LabelSelectorAsSelector(p.Spec.Selector)
	if err != nil || sel.Empty() {
		// An empty selector would match everything; require at least one term.
		return false
	}
	return sel.Matches(labels.Set(svc.Labels))
}

// AttachedTo reports whether svc runs the plugin, from either side.
func AttachedTo(p *koptanv1.CIPlugin, svc *koptanv1.Service) bool {
	for _, ref := range svc.Spec.Plugins {
		if ref.Name == p.Name && svc.Namespace == p.Namespace {
			return true
		}
	}
	return Targets(p, svc)
}

// Refs pins the plugins at their current generation for a CI spec.
func Refs(list []koptanv1.CIPlugin) []koptanv1.CIPluginRef {
	if len(list) == 0 {
		return nil
	}
	out := make([]koptanv1.CIPluginRef, 0, len(list))
	for _, p := range list {
		out = append(out, koptanv1.CIPluginRef{Name: p.Name, Generation: p.Generation})
	}
	return out
}
