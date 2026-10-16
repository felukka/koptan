package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// laggingCache misses every Pod, like an informer that has not seen it yet.
type laggingCache struct{ client.Client }

func (c laggingCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Pod); ok {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, key.Name)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

// quotaRejects refuses every Pod, like a namespace over its ResourceQuota.
type quotaRejects struct{ client.Client }

func (c quotaRejects) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*corev1.Pod); ok {
		return apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, obj.GetName(),
			errExceededQuota)
	}
	return c.Client.Create(ctx, obj, opts...)
}

var errExceededQuota = &quotaErr{}

type quotaErr struct{}

func (*quotaErr) Error() string { return "exceeded quota: tiny, requested: limits.memory=1Gi" }

var _ = Describe("Pipeline status", func() {
	const ns = "default"
	const sha = "0123456789abcdef0123456789abcdef01234567"

	newCI := func(name, svc string) *koptanv1.CI {
		ci := &koptanv1.CI{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: map[string]string{labelService: svc}},
			Spec: koptanv1.CISpec{
				Service:             koptanv1.NamespacedObjectReference{Name: svc},
				Registry:            koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "koptan/" + svc},
				Revision:            sha,
				DockerfileConfigMap: svc + "-dockerfile",
			},
		}
		Expect(k8sClient.Create(ctx, ci)).To(Succeed())
		return ci
	}

	// P-15: a reconcile from a stale cache must not fail a build whose Pod
	// it simply has not seen, nor one a newer build already replaced.
	It("does not report a build Pod lost because of a stale cache", func() {
		ci := newCI("stale-ci", "stale")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "stale-ci-build-new", Namespace: ns},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		ci.Status.BuildPod = pod.Name
		Expect(k8sClient.Status().Update(ctx, ci)).To(Succeed())

		r := &CIReconciler{Client: laggingCache{k8sClient}, Scheme: k8sClient.Scheme(), APIReader: k8sClient}
		got, lost, err := r.buildPodOf(ctx, ci)
		Expect(err).NotTo(HaveOccurred())
		Expect(lost).To(BeFalse())
		Expect(got.Name).To(Equal(pod.Name), "a Pod missing only from the cache is read from the API server")

		stale := ci.DeepCopy()
		stale.Status.BuildPod = "stale-ci-build-old" // deleted when the new build started
		got, lost, err = r.buildPodOf(ctx, stale)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeNil())
		Expect(lost).To(BeFalse(), "the CI already names a newer Pod")

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		_, lost, err = r.buildPodOf(ctx, ci)
		Expect(err).NotTo(HaveOccurred())
		Expect(lost).To(BeTrue(), "the Pod the CI still names is really gone")
	})

	It("refuses a status write based on a stale copy", func() {
		ci := newCI("lock-ci", "lock")
		stale := ci.DeepCopy()
		ci.Status.BuildPod = "lock-ci-build-new"
		Expect(k8sClient.Status().Update(ctx, ci)).To(Succeed())

		r := NewCIReconciler(k8sClient, k8sClient.Scheme())
		write := stale.DeepCopy()
		r.markCIFailed(write, "BuildPodLost", "build pod lock-ci-build-old disappeared")
		Expect(apierrors.IsConflict(r.patchStatus(ctx, stale, write))).To(BeTrue())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ci), ci)).To(Succeed())
		Expect(ci.Status.Phase).NotTo(Equal(koptanv1.CIPhaseFailed))
		Expect(ci.Status.BuildPod).To(Equal("lock-ci-build-new"))
	})

	// P-18: a rejected build Pod shows in the CI status and is tried again.
	It("reports a rejected build Pod and retries it", func() {
		Expect(k8sClient.Create(ctx, &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "quota", Namespace: ns},
			Spec:       koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/q.git"}},
		})).To(Succeed())
		ci := newCI("quota-ci", "quota")

		r := NewCIReconciler(quotaRejects{k8sClient}, k8sClient.Scheme())
		req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(ci)}
		_, err := r.Reconcile(ctx, req) // finalizer
		Expect(err).NotTo(HaveOccurred())
		res, err := r.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(Equal(failedRetryInterval))

		Expect(k8sClient.Get(ctx, req.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseFailed))
		Expect(ci.Status.Message).To(ContainSubstring("exceeded quota"))
		Expect(meta.FindStatusCondition(ci.Status.Conditions, "Ready").Reason).To(Equal("PodCreateFailed"))
		Expect(ci.Status.ObservedGeneration).NotTo(Equal(ci.Generation), "the build must not count as done")

		// Once the quota allows it, the next reconcile starts the build.
		ok := NewCIReconciler(k8sClient, k8sClient.Scheme())
		_, err = ok.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, req.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseBuilding))
	})

	// P-03: the Service phase follows its CI and CD.
	It("is Building until the CD runs the latest revision, and Failed with the CI or CD message", func() {
		svc := &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "follow", Namespace: ns},
			Spec:       koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/f.git"}},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		svc.Status.LatestRevision, svc.Status.CIRef = sha, "follow-ci"
		ci := newCI("follow-ci", "follow")
		sr := &ServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		phase := func() koptanv1.ServicePhase {
			Expect(sr.setPipelinePhase(ctx, svc)).To(Succeed())
			return svc.Status.Phase
		}
		setCI := func(p koptanv1.CIPhase, rev, msg string) {
			ci.Status.Phase, ci.Status.Revision, ci.Status.Message = p, rev, msg
			Expect(k8sClient.Status().Update(ctx, ci)).To(Succeed())
		}

		setCI(koptanv1.CIPhaseBuilding, "", "Building")
		Expect(phase()).To(Equal(koptanv1.ServicePhaseBuilding))

		setCI(koptanv1.CIPhaseFailed, "", "step 1: short-name resolution enforced")
		Expect(phase()).To(Equal(koptanv1.ServicePhaseFailed))
		Expect(svc.Status.Message).To(ContainSubstring("CI follow-ci failed: step 1: short-name resolution enforced"))
		Expect(pipelineFailure(svc)).To(BeTrue())
		Expect(meta.IsStatusConditionTrue(svc.Status.Conditions, "Ready")).To(BeFalse())

		setCI(koptanv1.CIPhaseSucceeded, sha, "build succeeded")
		Expect(phase()).To(Equal(koptanv1.ServicePhaseBuilding), "built, no CD yet")

		cd := &koptanv1.CD{
			ObjectMeta: metav1.ObjectMeta{Name: "follow-cd", Namespace: ns, Labels: map[string]string{labelService: "follow"}},
			Spec:       koptanv1.CDSpec{CI: koptanv1.NamespacedObjectReference{Name: "follow-ci"}},
		}
		Expect(k8sClient.Create(ctx, cd)).To(Succeed())
		svc.Status.CDRef = cd.Name
		setCD := func(p koptanv1.CDPhase, rev, msg string) {
			cd.Status.Phase, cd.Status.Revision, cd.Status.Message = p, rev, msg
			Expect(k8sClient.Status().Update(ctx, cd)).To(Succeed())
		}

		setCD(koptanv1.CDPhaseRunning, "an-older-revision", "1/1 replicas available")
		Expect(phase()).To(Equal(koptanv1.ServicePhaseBuilding), "still running the old revision")

		setCD(koptanv1.CDPhaseFailed, sha, "deployment is not progressing")
		Expect(phase()).To(Equal(koptanv1.ServicePhaseFailed))
		Expect(svc.Status.Message).To(ContainSubstring("CD follow-cd failed"))

		setCD(koptanv1.CDPhaseRunning, sha, "1/1 replicas available")
		Expect(phase()).To(Equal(koptanv1.ServicePhaseReady))
		Expect(meta.IsStatusConditionTrue(svc.Status.Conditions, "Ready")).To(BeTrue())

		// A new commit: the Service has moved on, its CI not yet.
		svc.Status.LatestRevision = "fedcba9876543210fedcba9876543210fedcba98"
		Expect(phase()).To(Equal(koptanv1.ServicePhaseBuilding))
	})

	It("keeps a Service with a failed CI discovered, so it only polls git", func() {
		svc := &koptanv1.Service{ObjectMeta: metav1.ObjectMeta{Generation: 2}}
		svc.Status.ObservedGeneration, svc.Status.CIRef = 2, "x-ci"
		setPipelineFailed(svc, reasonCIFailed, "CI x-ci failed")
		Expect(discovered(svc)).To(BeTrue())
		(&ServiceReconciler{}).markFailed(svc, "RevisionUnresolved", "branch gone")
		Expect(discovered(svc)).To(BeFalse(), "the Service's own failures rediscover")
	})

	// P-16: the Service reverts hand edits to its CI and recreates it.
	It("repairs a hand-edited CI and recreates a deleted one", func() {
		svc := &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "drift", Namespace: ns},
			Spec:       koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/d.git"}},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		svc.Status.LatestRevision, svc.Status.DockerfileConfigMap, svc.Status.ServiceType = sha, "drift-dockerfile", "go"
		sr := &ServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		Expect(sr.ensureOwnCI(ctx, svc, nil)).To(Succeed())
		key := types.NamespacedName{Name: svc.Status.CIRef, Namespace: ns}

		var ci koptanv1.CI
		Expect(k8sClient.Get(ctx, key, &ci)).To(Succeed())
		gen := ci.Generation
		Expect(sr.ensureOwnCI(ctx, svc, nil)).To(Succeed())
		Expect(k8sClient.Get(ctx, key, &ci)).To(Succeed())
		Expect(ci.Generation).To(Equal(gen), "an unchanged CI is not rewritten")

		ci.Spec.ContextDir = "hacked"
		Expect(k8sClient.Update(ctx, &ci)).To(Succeed())
		Expect(sr.ensureOwnCI(ctx, svc, nil)).To(Succeed())
		Expect(k8sClient.Get(ctx, key, &ci)).To(Succeed())
		Expect(ci.Spec.ContextDir).To(BeEmpty())

		Expect(k8sClient.Delete(ctx, &ci)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, key, &koptanv1.CI{}))
		}).Should(BeTrue())
		Expect(sr.ensureOwnCI(ctx, svc, nil)).To(Succeed())
		Expect(k8sClient.Get(ctx, key, &ci)).To(Succeed())
		Expect(ci.Spec.Revision).To(Equal(sha))
		Expect(ci.Spec.DockerfileConfigMap).To(Equal("drift-dockerfile"))
	})
})
