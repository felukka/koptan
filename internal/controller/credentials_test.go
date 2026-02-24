package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

// createCredentialSecret creates a Secret Koptan may use (labelled
// koptan.felukka.org/credentials=true) holding key "token", in "default".
func createCredentialSecret(name string) {
	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{labelCredentials: "true"}},
		StringData: map[string]string{"token": "t0ken"},
	})).To(Succeed())
}

// P-12: a Service, CI or plugin must not send an arbitrary Secret to a
// host its author chooses.
var _ = Describe("Credential Secrets", func() {
	const ns = "default"
	const sha = "0123456789abcdef0123456789abcdef01234567"

	createPlain := func(name string) {
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			StringData: map[string]string{"token": "SUPERSECRET"},
		})).To(Succeed())
	}

	It("refuses an unlabelled git token unless the policy allows it", func() {
		createPlain("db-password")
		svc := &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "exfil", Namespace: ns},
			Spec: koptanv1.ServiceSpec{Source: koptanv1.Source{
				Repo: "https://attacker.example.com/r.git",
				SecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "db-password"}, Key: "token"},
			}},
		}
		r := &ServiceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.gitToken(ctx, svc)
		Expect(err).To(MatchError(ContainSubstring("not labelled " + labelCredentials + "=true")))

		r.Secrets.AllowUnlabeled = true
		Expect(r.gitToken(ctx, svc)).To(Equal("SUPERSECRET"))
	})

	It("fails the CI and creates no Pod when a step references an unlabelled Secret", func() {
		createPlain("other-app-db")
		createCredentialSecret("ok-git")
		Expect(k8sClient.Create(ctx, &koptanv1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "sneaky", Namespace: ns},
			Spec: koptanv1.ServiceSpec{Source: koptanv1.Source{Repo: "https://git.example.com/s.git",
				SecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "ok-git"}, Key: "token"}}},
		})).To(Succeed())
		ci := &koptanv1.CI{
			ObjectMeta: metav1.ObjectMeta{Name: "sneaky-ci", Namespace: ns, Labels: map[string]string{labelService: "sneaky"}},
			Spec: koptanv1.CISpec{
				Service:             koptanv1.NamespacedObjectReference{Name: "sneaky"},
				Registry:            koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "koptan/sneaky"},
				Revision:            sha,
				DockerfileConfigMap: "sneaky-dockerfile",
				ExtraSteps: []corev1.Container{{Name: "peek", Image: "attacker/curl", EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "other-app-db"}},
				}}}},
			},
		}
		Expect(k8sClient.Create(ctx, ci)).To(Succeed())

		r := NewCIReconciler(k8sClient, k8sClient.Scheme())
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "sneaky-ci", Namespace: ns}}
		for range 2 { // finalizer, then the build
			_, err := r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(k8sClient.Get(ctx, req.NamespacedName, ci)).To(Succeed())
		Expect(ci.Status.Phase).To(Equal(koptanv1.CIPhaseFailed))
		Expect(ci.Status.Message).To(ContainSubstring(`secret "other-app-db" is not labelled`))
		var pods corev1.PodList
		Expect(k8sClient.List(ctx, &pods, client.MatchingLabels{labelCI: "sneaky-ci"})).To(Succeed())
		Expect(pods.Items).To(BeEmpty())
	})

	It("fails the CD and creates no Deployment when the app env references an unlabelled Secret", func() {
		createPlain("payroll-db")
		ci := &koptanv1.CI{
			ObjectMeta: metav1.ObjectMeta{Name: "envleak-ci", Namespace: ns},
			Spec: koptanv1.CISpec{
				Service:             koptanv1.NamespacedObjectReference{Name: "envleak"},
				Registry:            koptanv1.RegistrySpec{Registry: "ttl.sh", Repo: "koptan/envleak"},
				Revision:            sha,
				DockerfileConfigMap: "envleak-dockerfile",
			},
		}
		Expect(k8sClient.Create(ctx, ci)).To(Succeed())
		ci.Status.Image = "ttl.sh/koptan/envleak:0123456789ab"
		Expect(k8sClient.Status().Update(ctx, ci)).To(Succeed())
		cd := &koptanv1.CD{
			ObjectMeta: metav1.ObjectMeta{Name: "envleak-cd", Namespace: ns},
			Spec: koptanv1.CDSpec{
				CI: koptanv1.NamespacedObjectReference{Name: "envleak-ci"},
				Env: []corev1.EnvVar{{Name: "DB", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "payroll-db"}, Key: "token"}}}},
			},
		}
		Expect(k8sClient.Create(ctx, cd)).To(Succeed())

		r := &CDReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "envleak-cd", Namespace: ns}}
		for range 2 { // finalizer, then the deploy
			_, err := r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(k8sClient.Get(ctx, req.NamespacedName, cd)).To(Succeed())
		Expect(cd.Status.Phase).To(Equal(koptanv1.CDPhaseFailed))
		Expect(cd.Status.Message).To(ContainSubstring(`secret "payroll-db" is not labelled`))
		var deploy appsv1.Deployment
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, req.NamespacedName, &deploy))).To(BeTrue())
	})
})
