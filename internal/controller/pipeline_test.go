package controller

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// Drives CI -> build Pod -> CD -> Deployment by hand: envtest runs no
// kubelet or Deployment controller, so Pod and Deployment status are set
// directly.
var _ = Describe("Service pipeline", func() {
	const (
		ns  = "default"
		sha = "0123456789abcdef0123456789abcdef01234567"
	)
	replicas := int32(2)

	It("builds a revision in a Pod, then deploys it and follows readiness", func() {
		svc := &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "pipe", Namespace: ns},
			Spec: koptanv1.ServiceSpec{
				Source: koptanv1.Source{
					Repo:      "https://git.example.com/team/pipe.git",
					SecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "pipe-git"}, Key: "token"},
				},
				Env:      []corev1.EnvVar{{Name: "GREETING", Value: "hi"}},
				Replicas: &replicas,
				Port:     9090,
			},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		ci := &koptanv1.CI{
			ObjectMeta: metav1.ObjectMeta{Name: "pipe-ci", Namespace: ns, Labels: map[string]string{labelService: "pipe"}},
			Spec: koptanv1.CISpec{
				Service:             koptanv1.NamespacedObjectReference{Name: "pipe"},
				Registry:            koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "koptan/pipe"},
				Revision:            sha,
				DockerfileConfigMap: "pipe-dockerfile",
			},
		}
		Expect(k8sClient.Create(ctx, ci)).To(Succeed())

		ciR := NewCIReconciler(k8sClient, k8sClient.Scheme())
		ciReq := reconcile.Request{NamespacedName: types.NamespacedName{Name: "pipe-ci", Namespace: ns}}
		_, err := ciR.Reconcile(ctx, ciReq) // finalizer
		Expect(err).NotTo(HaveOccurred())
		_, err = ciR.Reconcile(ctx, ciReq) // start build
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, ciReq.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseBuilding))
		Expect(ci.Status.BuildPod).NotTo(BeEmpty())

		var pod corev1.Pod
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ci.Status.BuildPod, Namespace: ns}, &pod)).To(Succeed())
		clone := pod.Spec.InitContainers[0]
		Expect(clone.Command[2]).NotTo(ContainSubstring("git.example.com"), "repo must not be interpolated into the script")
		Expect(clone.Env).To(ContainElement(corev1.EnvVar{Name: "SHA", Value: sha}))
		Expect(clone.Env).To(ContainElement(HaveField("Name", "GIT_TOKEN")))
		build := pod.Spec.Containers[0]
		Expect(build.Env).To(ContainElement(corev1.EnvVar{Name: "IMAGE", Value: "ttl.sh/koptan/pipe:0123456789ab"}))

		// A second reconcile while the Pod runs must not start another build.
		_, err = ciR.Reconcile(ctx, ciReq)
		Expect(err).NotTo(HaveOccurred())
		var pods corev1.PodList
		Expect(k8sClient.List(ctx, &pods, client.InNamespace(ns), client.MatchingLabels{labelCI: "pipe-ci"})).To(Succeed())
		Expect(pods.Items).To(HaveLen(1))

		pod.Status.Phase = corev1.PodSucceeded
		Expect(k8sClient.Status().Update(ctx, &pod)).To(Succeed())
		_, err = ciR.Reconcile(ctx, ciReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, ciReq.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseSucceeded))
		Expect(ci.Status.Image).To(Equal("ttl.sh/koptan/pipe:0123456789ab"))
		Expect(ci.Status.Revision).To(Equal(sha))

		cd := &koptanv1.CD{}
		cdKey := types.NamespacedName{Name: "pipe-cd", Namespace: ns}
		Expect(k8sClient.Get(ctx, cdKey, cd)).To(Succeed())
		Expect(cd.Spec.Replicas).To(Equal(int32(2)))
		Expect(cd.Spec.Port).To(Equal(int32(9090)))
		Expect(cd.Spec.Env).To(ContainElement(corev1.EnvVar{Name: "GREETING", Value: "hi"}))

		cdR := &CDReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		cdReq := reconcile.Request{NamespacedName: cdKey}
		_, err = cdR.Reconcile(ctx, cdReq) // finalizer
		Expect(err).NotTo(HaveOccurred())
		_, err = cdR.Reconcile(ctx, cdReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, cdKey, cd)).To(Succeed())
		Expect(cd.Status.Phase).To(Equal(koptanv1.CDPhaseDeploying), "not Running before pods are available")

		var deploy appsv1.Deployment
		Expect(k8sClient.Get(ctx, cdKey, &deploy)).To(Succeed())
		Expect(*deploy.Spec.Replicas).To(Equal(int32(2)))
		app := deploy.Spec.Template.Spec.Containers[0]
		Expect(app.Image).To(Equal("ttl.sh/koptan/pipe:0123456789ab"))
		Expect(app.Ports[0].ContainerPort).To(Equal(int32(9090)))
		Expect(app.ReadinessProbe).NotTo(BeNil())
		Expect(app.Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: "9090"}))

		deploy.Status.ObservedGeneration = deploy.Generation
		deploy.Status.Replicas, deploy.Status.UpdatedReplicas = 2, 2
		deploy.Status.ReadyReplicas, deploy.Status.AvailableReplicas = 2, 2
		Expect(k8sClient.Status().Update(ctx, &deploy)).To(Succeed())
		_, err = cdR.Reconcile(ctx, cdReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, cdKey, cd)).To(Succeed())
		Expect(cd.Status.Phase).To(Equal(koptanv1.CDPhaseRunning))
		Expect(cd.Status.AvailableReplicas).To(Equal(int32(2)))

		// Scaling the CD is applied to the Deployment even while Running.
		cd.Spec.Replicas = 3
		Expect(k8sClient.Update(ctx, cd)).To(Succeed())
		_, err = cdR.Reconcile(ctx, cdReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, cdKey, &deploy)).To(Succeed())
		Expect(*deploy.Spec.Replicas).To(Equal(int32(3)))
		Expect(k8sClient.Get(ctx, cdKey, cd)).To(Succeed())
		Expect(cd.Status.Phase).To(Equal(koptanv1.CDPhaseDeploying))
	})

	It("reports a failed build once and does not rebuild the same revision", func() {
		Expect(k8sClient.Create(ctx, &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "fail", Namespace: ns},
			Spec:       koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/team/fail.git"}},
		})).To(Succeed())
		ci := &koptanv1.CI{
			ObjectMeta: metav1.ObjectMeta{Name: "fail-ci", Namespace: ns},
			Spec: koptanv1.CISpec{
				Service:             koptanv1.NamespacedObjectReference{Name: "fail"},
				Registry:            koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "koptan/fail"},
				Revision:            sha,
				DockerfileConfigMap: "fail-dockerfile",
			},
		}
		Expect(k8sClient.Create(ctx, ci)).To(Succeed())
		ciR := NewCIReconciler(k8sClient, k8sClient.Scheme())
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "fail-ci", Namespace: ns}}
		for range 2 {
			_, err := ciR.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(k8sClient.Get(ctx, req.NamespacedName, ci)).To(Succeed())
		var pod corev1.Pod
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ci.Status.BuildPod, Namespace: ns}, &pod)).To(Succeed())
		pod.Status.Phase = corev1.PodFailed
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name: "build-push",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 125, Message: "Error: pushing image: access denied"}},
		}}
		Expect(k8sClient.Status().Update(ctx, &pod)).To(Succeed())

		for range 3 {
			_, err := ciR.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(k8sClient.Get(ctx, req.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseFailed))
		Expect(ci.Status.Message).To(ContainSubstring("access denied"))
		var pods corev1.PodList
		Expect(k8sClient.List(ctx, &pods, client.InNamespace(ns), client.MatchingLabels{labelCI: "fail-ci"})).To(Succeed())
		Expect(pods.Items).To(HaveLen(1), "a failed revision must not be rebuilt in a loop")

		// A new revision starts a new build.
		ci.Spec.Revision = strings.Repeat("b", 40)
		Expect(k8sClient.Update(ctx, ci)).To(Succeed())
		_, err := ciR.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, req.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseBuilding))
		Expect(ci.Status.BuildingRevision).To(Equal(strings.Repeat("b", 40)))
	})
})
