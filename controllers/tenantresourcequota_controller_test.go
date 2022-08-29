package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
	"github.com/cybozu-go/necotiator/pkg/constants"
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

func newNamespace(name, teamName string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"team": teamName,
			},
		},
	}
}

func newTenantResourceQuota(name, teamName string) *necotiatorv1beta1.TenantResourceQuota {
	return &necotiatorv1beta1.TenantResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Finalizers: []string{
				constants.Finalizer,
			},
		},
		Spec: necotiatorv1beta1.TenantResourceQuotaSpec{
			Hard: v1.ResourceList{
				"limits.cpu": resource.MustParse("100m"),
			},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"team": teamName,
				},
			},
		},
	}
}

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
		tenantResourceQuotaName := newTestObjectName()
		teamName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, teamName)
		err := k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		name := newTestObjectName()
		namespace := newNamespace(name, teamName)
		err = k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: name, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))
			g.Expect(quota.Spec.Hard).Should(MatchAllKeys(Keys{
				corev1.ResourceName("limits.cpu"): SemanticEqual(resource.MustParse("0")),
			}))
		}).Should(Succeed())
	})

	It("should update TenantResourceQuota status", func() {
		tenantResourceQuotaName := newTestObjectName()
		teamName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, teamName)
		err := k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		name := newTestObjectName()
		namespace := newNamespace(name, teamName)
		err = k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		name2 := newTestObjectName()
		namespace2 := newNamespace(name2, teamName)
		err = k8sClient.Create(ctx, namespace2)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: name, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: name2, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())
		}).Should(Succeed())

		var quota corev1.ResourceQuota
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: name, Name: constants.ResourceQuotaNameDefault}, &quota)
		Expect(err).ShouldNot(HaveOccurred())
		quota.Status.Hard = corev1.ResourceList{
			"limits.cpu": resource.MustParse("0"),
		}
		quota.Status.Used = corev1.ResourceList{
			"limits.cpu": resource.MustParse("1"),
		}
		err = k8sClient.Status().Update(ctx, &quota)
		Expect(err).ShouldNot(HaveOccurred())

		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: name2, Name: constants.ResourceQuotaNameDefault}, &quota)
		Expect(err).ShouldNot(HaveOccurred())
		quota.Status.Hard = corev1.ResourceList{
			"limits.cpu": resource.MustParse("0"),
		}
		quota.Status.Used = corev1.ResourceList{
			"limits.cpu": resource.MustParse("100m"),
		}
		err = k8sClient.Status().Update(ctx, &quota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var tenantResourceQuota necotiatorv1beta1.TenantResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Name: tenantResourceQuotaName}, &tenantResourceQuota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(tenantResourceQuota.Status.Allocated).Should(MatchAllKeys(Keys{
				corev1.ResourceName("limits.cpu"): MatchAllFields(Fields{
					"Total": SemanticEqual(resource.MustParse("0")),
					"Namespaces": MatchAllKeys(Keys{
						name:  SemanticEqual(resource.MustParse("0")),
						name2: SemanticEqual(resource.MustParse("0")),
					}),
				}),
			}))
			g.Expect(tenantResourceQuota.Status.Used).Should(MatchAllKeys(Keys{
				corev1.ResourceName("limits.cpu"): MatchAllFields(Fields{
					"Total": SemanticEqual(resource.MustParse("1100m")),
					"Namespaces": MatchAllKeys(Keys{
						name:  SemanticEqual(resource.MustParse("1")),
						name2: SemanticEqual(resource.MustParse("100m")),
					}),
				}),
			}))
		}).Should(Succeed())
	})

	It("should create namespace before tenant resource", func() {
		namespaceName := newTestObjectName()
		teamName := newTestObjectName()
		namespace := newNamespace(namespaceName, teamName)
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, teamName)
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))
			g.Expect(quota.Spec.Hard).Should(MatchAllKeys(Keys{
				corev1.ResourceName("limits.cpu"): SemanticEqual(resource.MustParse("0")),
			}))
		}).Should(Succeed())

	})

	It("should create resource quota when namespace selector changed", func() {
		namespaceName := newTestObjectName()
		teamName := newTestObjectName()
		namespace := newNamespace(namespaceName, teamName)
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, newTestObjectName())
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Consistently(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).Should(Satisfy(errors.IsNotFound))
		}).WithTimeout(3*time.Second).Should(Succeed(), "resource quota created to the namespace with different label")

		tenantResourceQuota.Spec.NamespaceSelector.MatchLabels = map[string]string{
			"team": teamName,
		}
		err = k8sClient.Update(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))
			g.Expect(quota.Spec.Hard).Should(MatchAllKeys(Keys{
				corev1.ResourceName("limits.cpu"): SemanticEqual(resource.MustParse("0")),
			}))
		}).Should(Succeed())
	})

	It("should create resource quota when namespace label added", func() {
		namespaceName := newTestObjectName()
		teamName := newTestObjectName()
		namespace := newNamespace(namespaceName, "")
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, teamName)
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Consistently(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).Should(Satisfy(errors.IsNotFound))
		}).WithTimeout(3*time.Second).Should(Succeed(), "resource quota created to the namespace with different label")

		namespace.Labels = map[string]string{
			"team": teamName,
		}
		err = k8sClient.Update(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))
			g.Expect(quota.Spec.Hard).Should(MatchAllKeys(Keys{
				corev1.ResourceName("limits.cpu"): SemanticEqual(resource.MustParse("0")),
			}))
		}).Should(Succeed())
	})

	It("should delete resource quota label on deleting tenant resource quota", func() {
		namespaceName := newTestObjectName()
		teamName := newTestObjectName()
		namespace := newNamespace(namespaceName, teamName)
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, teamName)
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))
		}).Should(Succeed())

		err = k8sClient.Delete(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			err = k8sClient.Get(ctx, client.ObjectKey{Name: tenantResourceQuotaName}, tenantResourceQuota)
			g.Expect(err).Should(Satisfy(errors.IsNotFound))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(BeEmpty())
		}).Should(Succeed())
	})
})
