package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// Attaches CIPlugins to a Service from both sides and drives the CI build
// Pod by hand, as pipeline_test.go does.
var _ = Describe("CI plugins", func() {
	const (
		ns  = "default"
		sha = "1111111111111111111111111111111111111111"
	)

	customPlugin := func(name string, order int32, mutate func(*koptanv1.CIPluginSpec)) *koptanv1.CIPlugin {
		p := &koptanv1.CIPlugin{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: koptanv1.CIPluginSpec{
				Type:   koptanv1.CIPluginCustom,
				Order:  order,
				Custom: &koptanv1.CustomStep{Image: "alpine:3.20", Command: []string{"sh", "-c", "echo " + name}},
			},
		}
		if mutate != nil {
			mutate(&p.Spec)
		}
		return p
	}

	It("runs attached plugins between clone and build and rebuilds when one changes", func() {
		Expect(k8sClient.Create(ctx, customPlugin("plg-lint", 10, nil))).To(Succeed())
		Expect(k8sClient.Create(ctx, customPlugin("plg-scan", 5, func(s *koptanv1.CIPluginSpec) {
			s.TargetRefs = []koptanv1.PluginTargetRef{{Kind: "Service", Name: "plg"}}
		}))).To(Succeed())

		svc := &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "plg", Namespace: ns},
			Spec: koptanv1.ServiceSpec{
				Source:  koptanv1.Source{Repo: "https://git.example.com/team/plg.git"},
				Plugins: []koptanv1.PluginRef{{Name: "plg-lint"}},
			},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		svc.Status.CIRef = "plg-ci"
		svc.Status.ServiceType = "go"
		Expect(k8sClient.Status().Update(ctx, svc)).To(Succeed())

		ci := &koptanv1.CI{
			ObjectMeta: metav1.ObjectMeta{Name: "plg-ci", Namespace: ns},
			Spec: koptanv1.CISpec{
				Service:             koptanv1.NamespacedObjectReference{Name: "plg"},
				Registry:            koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "koptan/plg"},
				Revision:            sha,
				DockerfileConfigMap: "plg-dockerfile",
			},
		}
		Expect(k8sClient.Create(ctx, ci)).To(Succeed())

		// The Service resolves both plugins, by order, and pins them on its CI.
		sr := &ServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		refs, err := sr.resolvePlugins(ctx, svc)
		Expect(err).NotTo(HaveOccurred())
		Expect(refs).To(HaveLen(2))
		Expect(refs[0].Name).To(Equal("plg-scan"))
		Expect(refs[1].Name).To(Equal("plg-lint"))
		Expect(sr.syncPlugins(ctx, svc, refs)).To(Succeed())

		ciR := NewCIReconciler(k8sClient, k8sClient.Scheme())
		ciReq := reconcile.Request{NamespacedName: types.NamespacedName{Name: "plg-ci", Namespace: ns}}
		for range 2 { // finalizer, then the build
			_, err = ciR.Reconcile(ctx, ciReq)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(k8sClient.Get(ctx, ciReq.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.PluginResults).To(HaveLen(2))
		Expect(ci.Status.PluginResults[0].Phase).To(Equal(koptanv1.PluginPhasePending))

		var pod corev1.Pod
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ci.Status.BuildPod, Namespace: ns}, &pod)).To(Succeed())
		var names []string
		for _, c := range pod.Spec.InitContainers {
			names = append(names, c.Name)
		}
		Expect(names).To(Equal([]string{"clone", "plugin-plg-scan", "plugin-plg-lint"}))
		Expect(pod.Spec.Containers[0].Name).To(Equal("build-push"))
		Expect(pod.Spec.InitContainers[1].Env).To(ContainElement(corev1.EnvVar{Name: "KOPTAN_LANGUAGE", Value: "go"}))

		// The lint step fails: the CI fails and says which plugin and why.
		pod.Status.Phase = corev1.PodFailed
		pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
			{Name: "clone", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}},
			{Name: "plugin-plg-scan", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0, Message: "0 issues"}}},
			{Name: "plugin-plg-lint", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1, Message: "3 lint errors"}}},
		}
		Expect(k8sClient.Status().Update(ctx, &pod)).To(Succeed())
		_, err = ciR.Reconcile(ctx, ciReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, ciReq.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseFailed))
		Expect(ci.Status.Message).To(ContainSubstring("plugin-plg-lint failed (exit 1): 3 lint errors"))
		Expect(ci.Status.PluginResults[0]).To(Equal(koptanv1.PluginResult{
			Name: "plg-scan", Phase: koptanv1.PluginPhaseSucceeded, Message: "0 issues"}))
		Expect(ci.Status.PluginResults[1].Phase).To(Equal(koptanv1.PluginPhaseFailed))

		// Changing the plugin bumps its generation; the Service re-pins it,
		// which changes the CI spec, so the same revision builds again.
		var lint koptanv1.CIPlugin
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "plg-lint", Namespace: ns}, &lint)).To(Succeed())
		lint.Spec.Custom.Command = []string{"sh", "-c", "echo fixed"}
		Expect(k8sClient.Update(ctx, &lint)).To(Succeed())
		refs, err = sr.resolvePlugins(ctx, svc)
		Expect(err).NotTo(HaveOccurred())
		Expect(sr.syncPlugins(ctx, svc, refs)).To(Succeed())
		_, err = ciR.Reconcile(ctx, ciReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, ciReq.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseBuilding))
		Expect(ci.Status.BuildPod).NotTo(Equal(pod.Name))

		// The plugin reports itself Accepted and attached to the Service.
		pr := &CIPluginReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err = pr.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "plg-scan", Namespace: ns}})
		Expect(err).NotTo(HaveOccurred())
		var scan koptanv1.CIPlugin
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "plg-scan", Namespace: ns}, &scan)).To(Succeed())
		Expect(scan.Status.AttachedServices).To(Equal([]string{"plg"}))
		Expect(meta.IsStatusConditionTrue(scan.Status.Conditions, "Accepted")).To(BeTrue())
	})

	It("refuses to build when a named plugin is missing", func() {
		svc := &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "plg-missing", Namespace: ns},
			Spec: koptanv1.ServiceSpec{
				Source:  koptanv1.Source{Repo: "https://git.example.com/team/x.git"},
				Plugins: []koptanv1.PluginRef{{Name: "does-not-exist"}},
			},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		sr := &ServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := sr.resolvePlugins(ctx, svc)
		Expect(err).To(MatchError(ContainSubstring("does-not-exist not found")))
		Expect(k8sClient.Delete(ctx, svc, client.PropagationPolicy(metav1.DeletePropagationBackground))).To(Succeed())
	})
})
