package hooks

import (
	"context"
	"fmt"
	"sync"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	m           sync.Mutex
	testCounter int32
)

func newTestObjectName() string {
	m.Lock()
	defer m.Unlock()
	testCounter += 1
	return fmt.Sprintf("test-%d", testCounter)
}

var _ = Describe("Webhook Table Test", func() {

	type testCase struct {
		limit     corev1.ResourceList
		request   corev1.ResourceList
		allocated *map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage
		success   bool
		message   string
	}

	DescribeTable("Validator Test", func(testCase testCase) {
		namespaceName := newTestObjectName()
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: tenantResourceQuotaName,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: testCase.limit,
			},
		}
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuota.Status = necotiatorv1beta1.TenantResourceQuotaStatus{
			Allocated: allocated,
		}
		err = k8sClient.Status().Update(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		resourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: namespaceName,
				Labels: map[string]string{
					"app.kubernetes.io/created-by": "necotiator",
					"necotiator.cybozu.io/tenant":  tenantResourceQuotaName,
				},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: testCase.request,
			},
		}
		err = k8sClient.Create(ctx, resourceQuota)
		if testCase.success {
			Expect(err).ShouldNot(HaveOccurred())
		} else {
			Expect(err).Should(HaveStatusErrorReason(Equal(metav1.StatusReasonInvalid)))
			Expect(err).Should(HaveStatusErrorMessage(ContainSubstring(fmt.Sprintf(testCase.message, tenantResourceQuotaName))))
		}
	},
		Entry("should deny exceeded quota", testCase{limit: resource.MustParse("500m")}),
		Entry(""),
	)

})

var _ = Describe("Webhook Test", func() {
	ctx := context.Background()

	It("should deny exceeded quota", func() {
		namespaceName := newTestObjectName()
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: tenantResourceQuotaName,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"limits.cpu": resource.MustParse("500m"),
				},
			},
		}
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuota.Status = necotiatorv1beta1.TenantResourceQuotaStatus{
			Allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("0"),
					Namespaces: map[string]resource.Quantity{
						namespaceName: resource.MustParse("0"),
					},
				},
			},
		}
		err = k8sClient.Status().Update(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		resourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: namespaceName,
				Labels: map[string]string{
					"app.kubernetes.io/created-by": "necotiator",
					"necotiator.cybozu.io/tenant":  tenantResourceQuotaName,
				},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"limits.cpu": resource.MustParse("600m"),
				},
			},
		}
		err = k8sClient.Create(ctx, resourceQuota)
		Expect(err).Should(HaveStatusErrorReason(Equal(metav1.StatusReasonInvalid)))
		Expect(err).Should(HaveStatusErrorMessage(ContainSubstring(fmt.Sprintf("exceeded tenant quota: %s, requested: limits.cpu=600m, limited: limits.cpu=500m", tenantResourceQuotaName))))
	})

	It("should allow quota", func() {

		webhookTest()
		namespaceName := newTestObjectName()
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: tenantResourceQuotaName,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"limits.cpu": resource.MustParse("1000m"),
				},
			},
		}
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuota.Status = necotiatorv1beta1.TenantResourceQuotaStatus{
			Allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("0"),
					Namespaces: map[string]resource.Quantity{
						namespaceName: resource.MustParse("0"),
					},
				},
			},
		}
		err = k8sClient.Status().Update(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		resourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: namespaceName,
				Labels: map[string]string{
					"app.kubernetes.io/created-by": "necotiator",
					"necotiator.cybozu.io/tenant":  tenantResourceQuotaName,
				},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"limits.cpu": resource.MustParse("600m"),
				},
			},
		}
		err = k8sClient.Create(ctx, resourceQuota)
		Expect(err).ShouldNot(HaveOccurred())
	})

})
