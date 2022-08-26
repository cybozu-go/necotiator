package hooks

import (
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
		allocated map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage
		request   corev1.ResourceList
		allow     bool
		message   string
	}

	mainNamespace := "__MAIN_NAMESPACE__"

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

		for resourceName, usage := range testCase.allocated {
			if v, ok := usage.Namespaces[mainNamespace]; ok {
				usage.Namespaces[namespaceName] = v
				delete(usage.Namespaces, mainNamespace)
			}
			testCase.allocated[resourceName] = usage
		}
		tenantResourceQuota.Status = necotiatorv1beta1.TenantResourceQuotaStatus{
			Allocated: testCase.allocated,
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
		if testCase.allow {
			Expect(err).ShouldNot(HaveOccurred())
		} else {
			Expect(err).Should(HaveStatusErrorReason(Equal(metav1.StatusReasonInvalid)))
			Expect(err).Should(HaveStatusErrorMessage(ContainSubstring(fmt.Sprintf(testCase.message, tenantResourceQuotaName))))
		}
	},
		Entry("should deny exceeded quota", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("0"),
					Namespaces: map[string]resource.Quantity{
						mainNamespace: resource.MustParse("0"),
					},
				},
			},
			request: corev1.ResourceList{
				"limits.cpu": resource.MustParse("600m"),
			},
			message: "exceeded tenant quota: %s, requested: limits.cpu=600m, total: limits.cpu=600m, limited: limits.cpu=500m",
		}),
		Entry("should deny exceeded total quota", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("300m"),
					Namespaces: map[string]resource.Quantity{
						mainNamespace:       resource.MustParse("0"),
						newTestObjectName(): resource.MustParse("300m"),
					},
				},
			},
			request: corev1.ResourceList{
				"limits.cpu": resource.MustParse("300m"),
			},
			message: "exceeded tenant quota: %s, requested: limits.cpu=300m, total: limits.cpu=600m, limited: limits.cpu=500m",
		}),
		Entry("should deny exceeded total quota", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("0"),
					Namespaces: map[string]resource.Quantity{
						mainNamespace: resource.MustParse("0"),
					},
				},
			},
			request: corev1.ResourceList{},
			message: "required limits.cpu by tenant resource quota: %s",
		}),
		Entry("should deny exceeded quota if status is empty", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{},
			request: corev1.ResourceList{
				"limits.cpu": resource.MustParse("1"),
			},
			message: "exceeded tenant quota: %s, requested: limits.cpu=1, total: limits.cpu=1, limited: limits.cpu=500m",
		}),
		Entry("should allow quota less than limited", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("1"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("0"),
					Namespaces: map[string]resource.Quantity{
						mainNamespace: resource.MustParse("0"),
					},
				},
			},
			request: corev1.ResourceList{
				"limits.cpu": resource.MustParse("600m"),
			},
			allow: true,
		}),
		Entry("should allow increase quota", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("400m"),
					Namespaces: map[string]resource.Quantity{
						mainNamespace:       resource.MustParse("200m"),
						newTestObjectName(): resource.MustParse("200m"),
					},
				},
			},
			request: corev1.ResourceList{
				"limits.cpu": resource.MustParse("300m"),
			},
			allow: true,
		}),
		Entry("should allow zero quota on already exceeded", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("600m"),
					Namespaces: map[string]resource.Quantity{
						newTestObjectName(): resource.MustParse("600m"),
					},
				},
			},
			request: corev1.ResourceList{
				"limits.cpu": resource.MustParse("0"),
			},
			allow: true,
		}),
		Entry("should allow decrease resource on already exceeded", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("2"),
					Namespaces: map[string]resource.Quantity{
						mainNamespace:       resource.MustParse("1"),
						newTestObjectName(): resource.MustParse("600m"),
					},
				},
			},
			request: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allow: true,
		}),
		Entry("should allow unknown resource quota", testCase{
			limit: corev1.ResourceList{
				"limits.cpu": resource.MustParse("500m"),
			},
			allocated: map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("0"),
					Namespaces: map[string]resource.Quantity{
						mainNamespace: resource.MustParse("0"),
					},
				},
			},
			request: corev1.ResourceList{
				"limits.cpu":    resource.MustParse("0"),
				"limits.memory": resource.MustParse("1Gi"),
			},
			allow: true,
		}),
	)
})
