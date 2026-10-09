package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

var _ = Describe("SelfService", func() {
	const ns = "default"
	const key = "test-agent-key"

	secretRef := func(name string) corev1.SecretKeySelector {
		return corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: "token"}
	}
	reconcileTwice := func(r *SelfServiceReconciler, name string) {
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: ns}}
		for range 2 { // finalizer, then the work
			_, err := r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}
	}
	get := func(name string) *koptanv1.SelfService {
		ss := &koptanv1.SelfService{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, ss)).To(Succeed())
		return ss
	}

	It("runs a locked-down agent for an existing repo and creates its Service", func() {
		ss := &koptanv1.SelfService{
			ObjectMeta: metav1.ObjectMeta{Name: "shop", Namespace: ns},
			Spec: koptanv1.SelfServiceSpec{
				Repo: koptanv1.SelfServiceRepo{Existing: &koptanv1.ExistingRepo{
					URL: "https://git.example.com/team/shop.git", SecretRef: secretRef("shop-git")}},
				AI: koptanv1.AgentAI{Provider: "anthropic", Model: "claude-opus-5-5",
					APIKeySecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shop-ai"}, Key: "apiKey"}},
				Service: koptanv1.SelfServiceTemplate{Port: 3000},
			},
		}
		Expect(k8sClient.Create(ctx, ss)).To(Succeed())
		r := &SelfServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), AgentImage: "agent:test", AgentKey: key}
		reconcileTwice(r, "shop")

		ss = get("shop")
		Expect(ss.Status.Phase).To(Equal(koptanv1.SelfServicePhaseProvisioning))
		Expect(ss.Status.RepoURL).To(Equal("https://git.example.com/team/shop.git"))

		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "shop-agent", Namespace: ns}, &secret)).To(Succeed())
		Expect(string(secret.Data["token"])).To(Equal(AgentToken(key, ns, "shop")))

		var d appsv1.Deployment
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "shop-agent", Namespace: ns}, &d)).To(Succeed())
		pod := d.Spec.Template.Spec
		Expect(*pod.AutomountServiceAccountToken).To(BeFalse())
		c := pod.Containers[0]
		Expect(c.Image).To(Equal("agent:test"))
		Expect(*c.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
		Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: "KOPTAN_AGENT_APP_PORT", Value: "3000"}))
		for _, e := range c.Env {
			if e.Name == "KOPTAN_GIT_TOKEN" || e.Name == "KOPTAN_AI_API_KEY" || e.Name == "KOPTAN_AGENT_TOKEN" {
				Expect(e.Value).To(BeEmpty(), "%s must come from a Secret", e.Name)
				Expect(e.ValueFrom.SecretKeyRef).NotTo(BeNil())
			}
		}

		var svc koptanv1.Service
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "shop", Namespace: ns}, &svc)).To(Succeed())
		Expect(svc.Spec.Source.Repo).To(Equal("https://git.example.com/team/shop.git"))
		Expect(svc.Spec.Source.Revision).To(Equal("main"))
		Expect(svc.Spec.Source.SecretRef.Name).To(Equal("shop-git"))
		Expect(metav1.IsControlledBy(&svc, ss)).To(BeTrue())

		// Envtest runs no Deployment controller: mark the agent available.
		d.Status.Replicas, d.Status.ReadyReplicas, d.Status.AvailableReplicas = 1, 1, 1
		Expect(k8sClient.Status().Update(ctx, &d)).To(Succeed())
		reconcileTwice(r, "shop")
		Expect(get("shop").Status.Phase).To(Equal(koptanv1.SelfServicePhaseReady))
	})

	It("creates the repository on GitHub", func() {
		var created map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc("GET /user", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"login": "me"})
		})
		mux.HandleFunc("POST /orgs/team/repos", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&created)
			_ = json.NewEncoder(w).Encode(map[string]string{"clone_url": "https://github.com/team/new-app.git"})
		})
		gh := httptest.NewServer(mux)
		defer gh.Close()

		Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gh-token", Namespace: ns},
			StringData: map[string]string{"token": "ghp_x"}})).To(Succeed())
		Expect(k8sClient.Create(ctx, &koptanv1.SelfService{
			ObjectMeta: metav1.ObjectMeta{Name: "new-app", Namespace: ns},
			Spec: koptanv1.SelfServiceSpec{
				Repo: koptanv1.SelfServiceRepo{Create: &koptanv1.CreateRepo{Provider: koptanv1.GitProviderGitHub,
					Owner: "team", BaseURL: "https://api.github.example.com", TokenSecretRef: secretRef("gh-token")}},
				AI: koptanv1.AgentAI{Provider: "openai-compatible", Model: "llama3.1", BaseURL: "http://ollama:11434/v1"},
			},
		})).To(Succeed())

		// The CRD only allows https base URLs; point the fake server in afterwards.
		ss := get("new-app")
		ss.Spec.Repo.Create.BaseURL = gh.URL
		r := &SelfServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), AgentImage: "agent:test",
			AgentKey: key, HTTPClient: gh.Client()}
		Expect(r.ensureRepoFor(ss)).To(Equal("https://github.com/team/new-app.git"))
		Expect(created["name"]).To(Equal("new-app"))
		Expect(created["private"]).To(BeTrue())
	})

	It("fails clearly without an agent key or on a name clash", func() {
		Expect(k8sClient.Create(ctx, &koptanv1.Service{ObjectMeta: metav1.ObjectMeta{Name: "taken", Namespace: ns},
			Spec: koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/other.git"}}})).To(Succeed())
		Expect(k8sClient.Create(ctx, &koptanv1.SelfService{
			ObjectMeta: metav1.ObjectMeta{Name: "taken", Namespace: ns},
			Spec: koptanv1.SelfServiceSpec{
				Repo: koptanv1.SelfServiceRepo{Existing: &koptanv1.ExistingRepo{
					URL: "https://git.example.com/team/taken.git", SecretRef: secretRef("t")}},
				AI: koptanv1.AgentAI{Provider: "anthropic", Model: "claude-opus-5-5"},
			},
		})).To(Succeed())

		noKey := &SelfServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		reconcileTwice(noKey, "taken")
		Expect(get("taken").Status.Message).To(ContainSubstring("KOPTAN_AGENT_KEY"))

		r := &SelfServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), AgentImage: "a", AgentKey: key}
		reconcileTwice(r, "taken")
		ss := get("taken")
		Expect(ss.Status.Phase).To(Equal(koptanv1.SelfServicePhaseFailed))
		Expect(ss.Status.Message).To(ContainSubstring("does not belong to this SelfService"))
	})
})

// ensureRepoFor runs ensureRepo against a SelfService held in memory.
func (r *SelfServiceReconciler) ensureRepoFor(ss *koptanv1.SelfService) string {
	url, err := r.ensureRepo(ctx, ss)
	Expect(err).NotTo(HaveOccurred())
	return url
}
