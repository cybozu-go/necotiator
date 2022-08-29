package e2e

import (
	_ "embed"
	"encoding/json"

	"github.com/cybozu-go/necotiator/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
)

//go:embed testdata/tenantResourceQuota.yaml
var tenantResourceQuotaYAML []byte

var _ = Describe("test e2e necotiator", func() {
	It("should create tenant resource quota", func() {
		kubectlSafe(tenantResourceQuotaYAML, "apply", "-f", "-")
	})
	It("should create namespace", func() {
		kubectlSafe(nil, "create", "ns", "test1")
		kubectlSafe(nil, "label", "ns", "test1", "team=neco")
	})
	It("should create resource quota", func() {
		Eventually(func(g Gomega) {
			out, err := kubectl(nil, "get", "-n=test1", "quota", "default", "-o=json")
			g.Expect(err).ShouldNot(HaveOccurred())

			var quota corev1.ResourceQuota
			err = json.Unmarshal(out, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal("test-tenant"),
			}))
		}).Should(Succeed())
	})
	It("should deny resource quota label change", func() {
		_, err := kubectl(nil, "label", "quota", "-n=test1", "default", "necotiator.cybozu.io/tenant=other", "--overwrite")
		Expect(err).Should(HaveOccurred())
		Expect(err).Should(MatchError(ContainSubstring("tenant labels is immutable")))
	})
	It("should add finalizer to tenant resource quota", func() {
		Eventually(func(g Gomega) {
			out, err := kubectl(nil, "get", "tenantresourcequota", "test-tenant", "-o=json")
			g.Expect(err).ShouldNot(HaveOccurred())

			var quota necotiatorv1beta1.TenantResourceQuota
			err = json.Unmarshal(out, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Finalizers).Should(ContainElement(constants.Finalizer))
		}).Should(Succeed())
	})
})
