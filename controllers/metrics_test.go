package controllers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetricDesc struct {
	Name string
	Desc string
	Type prometheus.ValueType
}

type Metric struct {
	Name   string
	Labels map[string]string
	Value  float64
}

var _ = Describe("Test tenantResourceQuotaCollector", func() {
	ctx := context.Background()
	var collector *tenantResourceQuotaCollector
	var handler http.Handler

	getMetrics := func() map[string]float64 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/metrics", nil)
		handler.ServeHTTP(w, req)

		var parser expfmt.TextParser
		parsed, err := parser.TextToMetricFamilies(w.Body)
		ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

		result := make(map[string]float64)
		for _, mf := range parsed {
			for _, m := range mf.Metric {
				labels := make([]string, len(m.Label))
				for i, l := range m.Label {
					labels[i] = fmt.Sprintf("%s=%s", *l.Name, *l.Value)
				}
				key := *mf.Name + "{" + strings.Join(labels, ",") + "}"
				result[key] = *m.Gauge.Value
			}
		}
		return result
	}

	BeforeEach(func() {
		collector = &tenantResourceQuotaCollector{
			Client: k8sClient,
			ctx:    ctx,
		}

		registry := prometheus.NewPedanticRegistry()
		registry.MustRegister(collector)

		handler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	})

	It("should export necotiator_tenantresourcequota", func() {
		name := newTestObjectName()
		quota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: v1.ResourceList{
					"limits.cpu":      resource.MustParse("1"),
					"limits.memory":   resource.MustParse("100Mi"),
					"requests.memory": resource.MustParse("50M"),
				},
			},
		}
		err := k8sClient.Create(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		quota.Status = necotiatorv1beta1.TenantResourceQuotaStatus{
			Allocated: map[v1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("50m"),
				},
				"limits.memory": {
					Total: resource.MustParse("50Mi"),
				},
				"requests.memory": {
					Total: resource.MustParse("0"),
				},
			},
			Used: map[v1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("20m"),
				},
				"limits.memory": {
					Total: resource.MustParse("0"),
				},
				"requests.memory": {
					Total: resource.MustParse("0"),
				},
			},
		}
		err = k8sClient.Status().Update(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		metrics := getMetrics()
		Expect(metrics).Should(MatchKeys(IgnoreExtras, Keys{
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=hard}", name):           BeNumerically("==", 1),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=allocated}", name):      BeNumerically("==", 0.050),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=used}", name):           BeNumerically("==", 0.020),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=hard}", name):        BeNumerically("==", 100*1024*1024),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=allocated}", name):   BeNumerically("==", 50*1024*1024),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=used}", name):        BeNumerically("==", 0),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=requests.memory,tenantresourcequota=%s,type=hard}", name):      BeNumerically("==", 50*1000*1000),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=requests.memory,tenantresourcequota=%s,type=allocated}", name): BeNumerically("==", 0),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=requests.memory,tenantresourcequota=%s,type=used}", name):      BeNumerically("==", 0),
		}))
	})

	It("should not export necotiator_tenantresourcequota after deletion", func() {
		name := newTestObjectName()
		quota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: v1.ResourceList{
					"limits.cpu": resource.MustParse("100m"),
				},
			},
		}
		err := k8sClient.Create(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		quota.Status = necotiatorv1beta1.TenantResourceQuotaStatus{
			Allocated: map[v1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("50m"),
				},
			},
			Used: map[v1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("20m"),
				},
			},
		}
		err = k8sClient.Status().Update(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		metrics := getMetrics()
		Expect(metrics).Should(MatchKeys(IgnoreExtras, Keys{
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=hard}", name):      BeNumerically("==", 0.100),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=allocated}", name): BeNumerically("==", 0.050),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=used}", name):      BeNumerically("==", 0.020),
		}))

		err = k8sClient.Delete(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		metrics = getMetrics()
		Expect(metrics).ShouldNot(HaveKey(fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=hard}", name)))
		Expect(metrics).ShouldNot(HaveKey(fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=allocated}", name)))
		Expect(metrics).ShouldNot(HaveKey(fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=used}", name)))
	})

	It("should not export necotiator_tenantresourcequota for removed resource configurations", func() {
		name := newTestObjectName()
		quota := &necotiatorv1beta1.TenantResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
				Hard: v1.ResourceList{
					"limits.cpu":    resource.MustParse("100m"),
					"limits.memory": resource.MustParse("100Mi"),
				},
			},
		}
		err := k8sClient.Create(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		quota.Status = necotiatorv1beta1.TenantResourceQuotaStatus{
			Allocated: map[v1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("50m"),
				},
				"limits.memory": {
					Total: resource.MustParse("0"),
				},
			},
			Used: map[v1.ResourceName]necotiatorv1beta1.ResourceUsage{
				"limits.cpu": {
					Total: resource.MustParse("20m"),
				},
				"limits.memory": {
					Total: resource.MustParse("0"),
				},
			},
		}
		err = k8sClient.Status().Update(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		metrics := getMetrics()
		Expect(metrics).Should(MatchKeys(IgnoreExtras, Keys{
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=hard}", name):         BeNumerically("==", 0.100),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=allocated}", name):    BeNumerically("==", 0.050),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=used}", name):         BeNumerically("==", 0.020),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=hard}", name):      BeNumerically("==", 100*1024*1024),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=allocated}", name): BeNumerically("==", 0),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=used}", name):      BeNumerically("==", 0),
		}))

		delete(quota.Spec.Hard, "limits.memory")
		err = k8sClient.Update(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		delete(quota.Status.Allocated, "limits.memory")
		delete(quota.Status.Used, "limits.memory")
		err = k8sClient.Status().Update(ctx, quota)
		Expect(err).ShouldNot(HaveOccurred())

		metrics = getMetrics()
		Expect(metrics).Should(MatchKeys(IgnoreExtras, Keys{
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=hard}", name):      BeNumerically("==", 0.100),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=allocated}", name): BeNumerically("==", 0.050),
			fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.cpu,tenantresourcequota=%s,type=used}", name):      BeNumerically("==", 0.020),
		}))
		Expect(metrics).ShouldNot(HaveKey(fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=hard}", name)))
		Expect(metrics).ShouldNot(HaveKey(fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=allocated}", name)))
		Expect(metrics).ShouldNot(HaveKey(fmt.Sprintf("necotiator_tenantresourcequota{resource=limits.memory,tenantresourcequota=%s,type=used}", name)))
	})
})
