package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// labelCredentials marks a Secret that Koptan resources may use. The
// operator reads Secrets with its own permission and build Pods send them
// to URLs taken from the spec, so without this opt-in anyone allowed to
// create a Service, CIPlugin, CI or SelfService could have any Secret in
// the namespace sent to a host they choose.
const labelCredentials = "koptan.felukka.org/credentials"

// SecretPolicy decides which Secrets Koptan resources may reference.
type SecretPolicy struct {
	// AllowUnlabeled accepts Secrets without the credentials label
	// (--allow-unlabeled-secrets), the behaviour before the label existed.
	AllowUnlabeled bool
}

// check reports whether s may be used as credentials.
func (p SecretPolicy) check(s *corev1.Secret) error {
	if p.AllowUnlabeled || s.Labels[labelCredentials] == "true" {
		return nil
	}
	return fmt.Errorf("secret %q is not labelled %s=true; add the label to let Koptan use it",
		s.Name, labelCredentials)
}

// get reads the Secret ns/name and checks it may be used as credentials.
func (p SecretPolicy) get(ctx context.Context, c client.Reader, ns, name string) (*corev1.Secret, error) {
	var s corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &s); err != nil {
		return nil, fmt.Errorf("read secret %q: %w", name, err)
	}
	return &s, p.check(&s)
}

// checkPod checks every Secret the Pod references; see checkPodSpec.
func (p SecretPolicy) checkPod(ctx context.Context, c client.Reader, pod *corev1.Pod) error {
	return p.checkPodSpec(ctx, c, pod.Namespace, &pod.Spec)
}

// checkPodSpec checks every Secret a pod spec in ns references: container
// env and envFrom, Secret and projected volumes, and imagePullSecrets. An
// optional reference to a missing Secret is fine; the kubelet skips it.
func (p SecretPolicy) checkPodSpec(ctx context.Context, c client.Reader, ns string, spec *corev1.PodSpec) error {
	optional := map[string]bool{}
	var names []string
	add := func(name string, opt *bool) {
		if _, seen := optional[name]; !seen {
			names = append(names, name)
			optional[name] = true
		}
		optional[name] = optional[name] && opt != nil && *opt
	}
	containers := append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...)
	for _, ctr := range containers {
		for _, e := range ctr.Env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				add(e.ValueFrom.SecretKeyRef.Name, e.ValueFrom.SecretKeyRef.Optional)
			}
		}
		for _, e := range ctr.EnvFrom {
			if e.SecretRef != nil {
				add(e.SecretRef.Name, e.SecretRef.Optional)
			}
		}
	}
	for _, v := range spec.Volumes {
		if v.Secret != nil {
			add(v.Secret.SecretName, v.Secret.Optional)
		}
		if v.Projected != nil {
			for _, src := range v.Projected.Sources {
				if src.Secret != nil {
					add(src.Secret.Name, src.Secret.Optional)
				}
			}
		}
	}
	for _, ref := range spec.ImagePullSecrets {
		add(ref.Name, nil)
	}
	for _, name := range names {
		var s corev1.Secret
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &s)
		if apierrors.IsNotFound(err) && optional[name] {
			continue
		}
		if err != nil {
			return fmt.Errorf("read secret %q: %w", name, err)
		}
		if err := p.check(&s); err != nil {
			return err
		}
	}
	return nil
}
