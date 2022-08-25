package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
)

var _ = Describe("Test TenantResourceQuotaController", func() {
	ctx := context.Background()
	var stopFunc func()

	BeforeEach(func() {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:             scheme,
			LeaderElection:     false,
			MetricsBindAddress: "0",
		})
		Expect(err).ShouldNot(HaveOccurred())

		reconciler := &TenantResourceQuotaReconciler{
			Client: mgr.GetClient(),
			Scheme: scheme,
		}
		err = reconciler.SetupWithManager(ctx, mgr)
		Expect(err).ShouldNot(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			err := mgr.Start(ctx)
			if err != nil {
				panic(err)
			}
		}()
		time.Sleep(100 * time.Millisecond)
	})

	AfterEach(func() {
		stopFunc()
		time.Sleep(100 * time.Millisecond)
	})

	It("should create resource quota", func() {
		tenantResourceQuota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: v1.ResourceList{
					"limits.cpu": resource.MustParse("100m"),
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"team": "test",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Labels: map[string]string{
					"team": "test",
				},
			},
		}
		err = k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "test"}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(HaveKeyWithValue("app.kubernetes.io/created-by", "necotiator"))
			g.Expect(quota.Labels).Should(HaveKeyWithValue("necotiator.cybozu.io/tenant", "test"))
			g.Expect(quota.Spec.Hard["limits.cpu"]).Should(SemanticEqual(resource.MustParse("0")))
			g.Expect(quota.Spec.Hard).ShouldNot(HaveKey(BeEquivalentTo("limits.memory")))
		}).Should(Succeed())
	})
})
