package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/notify"
)

// hookServer collects webhook events and fails while failing is set.
type hookServer struct {
	mu      sync.Mutex
	events  []notify.Event
	failing bool
}

func (h *hookServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failing {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var e notify.Event
	_ = json.Unmarshal(body, &e)
	h.events = append(h.events, e)
}

func (h *hookServer) kinds() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []string{}
	for _, e := range h.events {
		out = append(out, e.Kind)
	}
	return out
}

var _ = Describe("Alert", func() {
	const ns = "default"
	const sha = "2222222222222222222222222222222222222222"

	It("sends each pipeline event once, after the state it was created in", func() {
		hook := &hookServer{}
		srv := httptest.NewServer(hook)
		defer srv.Close()

		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "alert-hook", Namespace: ns},
			StringData: map[string]string{"url": srv.URL},
		})).To(Succeed())

		svc := &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "alerted", Namespace: ns, Labels: map[string]string{"team": "a"}},
			Spec:       koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/a.git"}},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		svc.Status.LatestRevision = sha
		Expect(k8sClient.Status().Update(ctx, svc)).To(Succeed())

		ci := &koptanv1.CI{
			ObjectMeta: metav1.ObjectMeta{Name: "alerted-ci", Namespace: ns, Labels: map[string]string{labelService: "alerted"}},
			Spec: koptanv1.CISpec{Service: koptanv1.NamespacedObjectReference{Name: "alerted"},
				Registry: koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "a"}},
		}
		Expect(k8sClient.Create(ctx, ci)).To(Succeed())
		ci.Status = koptanv1.CIStatus{Phase: koptanv1.CIPhaseSucceeded, BuildingRevision: sha, BuildPod: "p1",
			Image: "ttl.sh/a:222"}
		Expect(k8sClient.Status().Update(ctx, ci)).To(Succeed())

		alert := &koptanv1.Alert{
			ObjectMeta: metav1.ObjectMeta{Name: "team-a", Namespace: ns},
			Spec: koptanv1.AlertSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}},
				Events:   []koptanv1.AlertEvent{koptanv1.AlertPush, koptanv1.AlertCIStarted, koptanv1.AlertCIFailed},
				Channels: []koptanv1.AlertChannel{{Name: "hook", Type: koptanv1.ChannelWebhook,
					URLSecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "alert-hook"}, Key: "url"}}},
			},
		}
		Expect(k8sClient.Create(ctx, alert)).To(Succeed())

		r := &AlertReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), HTTPClient: srv.Client()}
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "team-a", Namespace: ns}}
		reconcileAlert := func() {
			_, err := r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}

		// The state the Alert was created in is not news.
		reconcileAlert()
		Expect(hook.kinds()).To(BeEmpty())
		Expect(k8sClient.Get(ctx, req.NamespacedName, alert)).To(Succeed())
		Expect(alert.Status.Initialized).To(BeTrue())
		Expect(meta.IsStatusConditionTrue(alert.Status.Conditions, "Ready")).To(BeTrue())

		// A new build starts: one CIStarted, never repeated.
		ci.Status.Phase = koptanv1.CIPhaseBuilding
		ci.Status.BuildPod = "p2"
		Expect(k8sClient.Status().Update(ctx, ci)).To(Succeed())
		reconcileAlert()
		reconcileAlert()
		Expect(hook.kinds()).To(Equal([]string{"CIStarted"}))

		// It succeeds, which this Alert does not send; then a push arrives
		// while the receiver is down: the delivery fails and is retried.
		ci.Status.Phase = koptanv1.CIPhaseSucceeded
		Expect(k8sClient.Status().Update(ctx, ci)).To(Succeed())
		hook.failing = true
		svc.Status.LatestRevision = "3333333333333333333333333333333333333333"
		Expect(k8sClient.Status().Update(ctx, svc)).To(Succeed())
		res, err := r.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(Equal(alertRetryInterval))
		Expect(k8sClient.Get(ctx, req.NamespacedName, alert)).To(Succeed())
		Expect(meta.IsStatusConditionFalse(alert.Status.Conditions, "Delivered")).To(BeTrue())
		Expect(alert.Status.Deliveries[0].Success).To(BeFalse())

		hook.failing = false
		reconcileAlert()
		Expect(hook.kinds()).To(Equal([]string{"CIStarted", "Push"}))
		Expect(hook.events[1].Revision).To(HavePrefix("3333"))
		Expect(k8sClient.Get(ctx, req.NamespacedName, alert)).To(Succeed())
		Expect(alert.Status.Deliveries[0].Success).To(BeTrue())
		Expect(meta.IsStatusConditionTrue(alert.Status.Conditions, "Delivered")).To(BeTrue())
	})

	It("reports a channel whose Secret is missing", func() {
		alert := &koptanv1.Alert{
			ObjectMeta: metav1.ObjectMeta{Name: "no-secret", Namespace: ns},
			Spec: koptanv1.AlertSpec{
				ServiceRef: &koptanv1.NamespacedObjectReference{Name: "alerted"},
				Channels: []koptanv1.AlertChannel{{Name: "slack", Type: koptanv1.ChannelSlack,
					URLSecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "missing"}, Key: "url"}}},
			},
		}
		Expect(k8sClient.Create(ctx, alert)).To(Succeed())
		r := &AlertReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), HTTPClient: http.DefaultClient}
		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "no-secret", Namespace: ns}})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(Equal(alertRetryInterval))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "no-secret", Namespace: ns}, alert)).To(Succeed())
		cond := meta.FindStatusCondition(alert.Status.Conditions, "Ready")
		Expect(cond.Reason).To(Equal("ChannelUnavailable"))
	})
})
