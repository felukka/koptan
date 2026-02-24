package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha "github.com/felukka/koptan/api/v1alpha"
)

var _ = Describe("Slipway Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-slipway"

		ctx := context.Background()

		nn := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			var existing v1alpha.Slipway
			err := k8sClient.Get(ctx, nn, &existing)
			if err != nil && errors.IsNotFound(err) {
				resource := &v1alpha.Slipway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: v1alpha.SlipwaySpec{
						AppRef: v1alpha.AppRef{
							Name: "test-goapp",
							Kind: "GoApp",
						},
						Image: v1alpha.ImageSpec{
							Registry: "ghcr.io",
							Name:     "example/app",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha.Slipway{}
			err := k8sClient.Get(ctx, nn, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			reconciler := &SlipwayReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nn,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
