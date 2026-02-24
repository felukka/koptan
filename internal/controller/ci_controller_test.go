package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	koptanv1 "github.com/felukka/koptan/api/v1"
)

var _ = Describe("CI Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-ci"

		ctx := context.Background()

		nn := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			var existing koptanv1.CI
			err := k8sClient.Get(ctx, nn, &existing)
			if err != nil && errors.IsNotFound(err) {
				resource := &koptanv1.CI{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: koptanv1.CISpec{
						Service: koptanv1.NamespacedObjectReference{
							Name: "test-service",
						},
						Registry: koptanv1.RegistrySpec{
							Registry: "ghcr.io",
							Repo:     "felukka/test",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &koptanv1.CI{}
			err := k8sClient.Get(ctx, nn, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			reconciler := NewCIReconciler(k8sClient, k8sClient.Scheme())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nn,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
