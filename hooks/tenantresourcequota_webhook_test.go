package hooks

import (
	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
	"github.com/cybozu-go/necotiator/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("TenantResourceQuota Webhook Test", func() {
	It("should add finalizer to tenant resource quota", func() {
		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: tenantResourceQuotaName,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: corev1.ResourceList{},
			},
		}
		err := k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		err = k8sClient.Get(ctx, client.ObjectKey{Name: tenantResourceQuotaName}, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(tenantResourceQuota.Finalizers).Should(ContainElement(constants.Finalizer))
	})

	It("should not add finalizer to tenant resource quota on update", func() {
		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: tenantResourceQuotaName,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: corev1.ResourceList{},
			},
		}
		err := k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		err = k8sClient.Get(ctx, client.ObjectKey{Name: tenantResourceQuotaName}, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		controllerutil.RemoveFinalizer(tenantResourceQuota, constants.Finalizer)
		err = k8sClient.Update(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		err = k8sClient.Get(ctx, client.ObjectKey{Name: tenantResourceQuotaName}, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(tenantResourceQuota.Finalizers).ShouldNot(ContainElement(constants.Finalizer))
	})
})
