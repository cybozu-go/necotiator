package controllers

import (
	"context"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	tenantResourceQuotaDesc = prometheus.NewDesc(
		"necotiator_tenantresourcequota",
		"Information about tenant resource quota",
		[]string{"tenantresourcequota", "resource", "type"}, nil)
)

type tenantResourceQuotaCollector struct {
	client.Client
	ctx context.Context
}

func (c *tenantResourceQuotaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- tenantResourceQuotaDesc
}

func (c *tenantResourceQuotaCollector) Collect(ch chan<- prometheus.Metric) {
	var quotaList necotiatorv1beta1.TenantResourceQuotaList
	if err := c.List(c.ctx, &quotaList); err != nil {
		log.FromContext(c.ctx).Error(err, "Unable to list tenant resource quota for collecting metrics")
		return
	}
	for _, quota := range quotaList.Items {
		for resourceName, v := range quota.Spec.Hard {
			ch <- prometheus.MustNewConstMetric(
				tenantResourceQuotaDesc,
				prometheus.GaugeValue,
				float64(v.MilliValue())/1000,
				quota.Name, string(resourceName), "hard",
			)
		}
		for resourceName, v := range quota.Status.Allocated {
			ch <- prometheus.MustNewConstMetric(
				tenantResourceQuotaDesc,
				prometheus.GaugeValue,
				float64(v.Total.MilliValue())/1000,
				quota.Name, string(resourceName), "allocated",
			)
		}
		for resourceName, v := range quota.Status.Used {
			ch <- prometheus.MustNewConstMetric(
				tenantResourceQuotaDesc,
				prometheus.GaugeValue,
				float64(v.Total.MilliValue())/1000,
				quota.Name, string(resourceName), "used",
			)
		}
	}
}

func SetupMetrics(ctx context.Context, c client.Client) error {
	return metrics.Registry.Register(&tenantResourceQuotaCollector{Client: c, ctx: ctx})
}
