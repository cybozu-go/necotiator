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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"
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
			Hard: corev1.ResourceList{
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

	It("should delete resource quota label on updating tenant resource quota label selector", func() {
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

		tenantResourceQuota.Spec.NamespaceSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"team": "alternativeTeam",
			},
		}

		err = k8sClient.Update(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var quota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(BeEmpty())
		}).Should(Succeed())
	})

	type testCase struct {
		initialTenantQuota  corev1.ResourceList
		generatedQuota      corev1.ResourceList
		modifiedQuota       corev1.ResourceList
		modifiedTenantQuota corev1.ResourceList
		updatedQuota        corev1.ResourceList
		SSA                 bool
	}

	DescribeTable("Tenant Resource Quota Editor Test", func(testCase testCase) {
		namespaceName := newTestObjectName()
		teamName := newTestObjectName()
		namespace := newNamespace(namespaceName, teamName)
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, teamName)
		tenantResourceQuota.Spec.Hard = testCase.initialTenantQuota

		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		var quota corev1.ResourceQuota
		Eventually(func(g Gomega) {
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))

			g.Expect(quota.Spec.Hard).Should(SemanticEqual(testCase.generatedQuota))
		}).Should(Succeed())

		if testCase.SSA {
			quotaApply := applycorev1.ResourceQuota(quota.GetName(), quota.GetNamespace()).
				WithLabels(quota.GetLabels()).
				WithSpec(applycorev1.ResourceQuotaSpec().WithHard(testCase.modifiedQuota))

			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(quotaApply)
			Expect(err).ShouldNot(HaveOccurred())
			patch := &unstructured.Unstructured{
				Object: obj,
			}

			err = k8sClient.Patch(ctx, patch, client.Apply, &client.PatchOptions{
				FieldManager: "kubectl",
				Force:        pointer.Bool(true),
			})
			Expect(err).ShouldNot(HaveOccurred())
		} else {
			quota.Spec.Hard = testCase.modifiedQuota
			err = k8sClient.Update(ctx, &quota)
			Expect(err).ShouldNot(HaveOccurred())
		}

		tenantResourceQuota.Spec.Hard = testCase.modifiedTenantQuota
		err = k8sClient.Update(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		if !testCase.SSA {
			Eventually(func(g Gomega) {
				var quotaAfter corev1.ResourceQuota
				err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quotaAfter)
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(quota.ResourceVersion).ShouldNot(Equal(quotaAfter.ResourceVersion))
			}).Should(Succeed())

			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			fmt.Println("466here", quota.ResourceVersion)
			Expect(err).ShouldNot(HaveOccurred())
			quota.Spec.Hard = testCase.modifiedQuota
			err = k8sClient.Update(ctx, &quota)
			fmt.Println("470updatehere", quota.ResourceVersion, quota)
			Expect(err).ShouldNot(HaveOccurred())
			Consistently(func(g Gomega) {
				var quotaAfter corev1.ResourceQuota
				err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quotaAfter)
				g.Expect(err).ShouldNot(HaveOccurred())

				fmt.Println("477get here", quotaAfter)
				g.Expect(quota.ResourceVersion).Should(Equal(quotaAfter.ResourceVersion))
			}).Should(Succeed())
		}

		Eventually(func(g Gomega) {
			quota = corev1.ResourceQuota{}
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))

			g.Expect(quota.Spec.Hard).Should(SemanticEqual(testCase.updatedQuota))
		}).Should(Succeed())

	}, Entry("should add new resource to resource quota after adding tenant resource quota resource and not overwrite current resource quota by clinet side apply", testCase{
		initialTenantQuota: corev1.ResourceList{
			corev1.ResourceName("limits.cpu"):   resource.MustParse("100m"),
			corev1.ResourceName("requests.cpu"): resource.MustParse("100m"),
		},
		generatedQuota: corev1.ResourceList{
			corev1.ResourceName("limits.cpu"):   resource.MustParse("0"),
			corev1.ResourceName("requests.cpu"): resource.MustParse("0"),
		},
		modifiedQuota: corev1.ResourceList{
			corev1.ResourceName("limits.cpu"):   resource.MustParse("50m"),
			corev1.ResourceName("requests.cpu"): resource.MustParse("0"),
		},
		modifiedTenantQuota: corev1.ResourceList{
			corev1.ResourceName("limits.cpu"):    resource.MustParse("100m"),
			corev1.ResourceName("requests.cpu"):  resource.MustParse("100m"),
			corev1.ResourceName("limits.memory"): resource.MustParse("100Mi"),
		},
		updatedQuota: corev1.ResourceList{
			corev1.ResourceName("limits.cpu"):    resource.MustParse("50m"),
			corev1.ResourceName("requests.cpu"):  resource.MustParse("0"),
			corev1.ResourceName("limits.memory"): resource.MustParse("0"),
		},
		SSA: false,
	}),
		Entry("should delete old resource quota not edited by user after deleting tenant resource quota tenant resource quota resource by client side apply", testCase{
			initialTenantQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):      resource.MustParse("100m"),
				corev1.ResourceName("requests.cpu"):    resource.MustParse("100m"),
				corev1.ResourceName("requests.memory"): resource.MustParse("500Mi"),
			},
			generatedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):      resource.MustParse("0"),
				corev1.ResourceName("requests.cpu"):    resource.MustParse("0"),
				corev1.ResourceName("requests.memory"): resource.MustParse("0"),
			},
			modifiedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):      resource.MustParse("50m"),
				corev1.ResourceName("requests.cpu"):    resource.MustParse("0"),
				corev1.ResourceName("requests.memory"): resource.MustParse("0"),
			},
			modifiedTenantQuota: corev1.ResourceList{
				corev1.ResourceName("requests.cpu"): resource.MustParse("0"),
			},
			updatedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):   resource.MustParse("50m"),
				corev1.ResourceName("requests.cpu"): resource.MustParse("0"),
			},
			SSA: false,
		}),
		Entry("should add new resource to resource quota after adding tenant resource quota resource and not overwrite current resource quota by server side apply", testCase{
			initialTenantQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):   resource.MustParse("100m"),
				corev1.ResourceName("requests.cpu"): resource.MustParse("100m"),
			},
			generatedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):   resource.MustParse("0"),
				corev1.ResourceName("requests.cpu"): resource.MustParse("0"),
			},
			modifiedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"): resource.MustParse("50m"),
			},
			modifiedTenantQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):    resource.MustParse("100m"),
				corev1.ResourceName("requests.cpu"):  resource.MustParse("100m"),
				corev1.ResourceName("limits.memory"): resource.MustParse("100Mi"),
			},
			updatedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):    resource.MustParse("50m"),
				corev1.ResourceName("requests.cpu"):  resource.MustParse("0"),
				corev1.ResourceName("limits.memory"): resource.MustParse("0"),
			},
			SSA: true,
		}),
		Entry("should delete old resource quota not edited by user after deleting tenant resource quota tenant resource quota resource by server side apply ", testCase{
			initialTenantQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):      resource.MustParse("100m"),
				corev1.ResourceName("requests.cpu"):    resource.MustParse("100m"),
				corev1.ResourceName("requests.memory"): resource.MustParse("500Mi"),
			},
			generatedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):      resource.MustParse("0"),
				corev1.ResourceName("requests.cpu"):    resource.MustParse("0"),
				corev1.ResourceName("requests.memory"): resource.MustParse("0"),
			},
			modifiedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"): resource.MustParse("50m"),
			},
			modifiedTenantQuota: corev1.ResourceList{
				corev1.ResourceName("requests.cpu"): resource.MustParse("0"),
			},
			updatedQuota: corev1.ResourceList{
				corev1.ResourceName("limits.cpu"):   resource.MustParse("50m"),
				corev1.ResourceName("requests.cpu"): resource.MustParse("0"),
			},
			SSA: true,
		}),
	)

	It("should change resource quota label on updating tenant resource quota label selector", func() {
		namespaceName := newTestObjectName()
		teamName := newTestObjectName()
		namespace := newNamespace(namespaceName, teamName)
		err := k8sClient.Create(ctx, namespace)
		Expect(err).ShouldNot(HaveOccurred())

		tenantResourceQuotaName := newTestObjectName()
		tenantResourceQuota := newTenantResourceQuota(tenantResourceQuotaName, teamName)
		err = k8sClient.Create(ctx, tenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		var quota corev1.ResourceQuota
		Eventually(func(g Gomega) {
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &quota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(tenantResourceQuotaName),
			}))
		}).Should(Succeed())

		migrationNameSpaceName := newTestObjectName()
		migrationTeamName := newTestObjectName()
		migrationNameSpace := newNamespace(migrationNameSpaceName, migrationTeamName)
		err = k8sClient.Create(ctx, migrationNameSpace)
		Expect(err).ShouldNot(HaveOccurred())

		migrationTenantResourceQuotaName := newTestObjectName()
		migrationTenantResourceQuota := newTenantResourceQuota(migrationTenantResourceQuotaName, migrationTeamName)
		err = k8sClient.Create(ctx, migrationTenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var migrateQuota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: migrationNameSpaceName, Name: constants.ResourceQuotaNameDefault}, &migrateQuota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(migrateQuota.Labels).Should(MatchAllKeys(Keys{
				constants.LabelCreatedBy: Equal(constants.CreatedBy),
				constants.LabelTenant:    Equal(migrationTenantResourceQuotaName),
			}))
		}).Should(Succeed())

		migrationTenantResourceQuota.Spec.NamespaceSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"team": teamName,
			},
		}
		err = k8sClient.Update(ctx, migrationTenantResourceQuota)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			var migrateQuota corev1.ResourceQuota
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: constants.ResourceQuotaNameDefault}, &migrateQuota)
			g.Expect(err).ShouldNot(HaveOccurred())

			g.Expect(quota.Labels).Should(Equal(migrateQuota.Labels))
		}).Should(Succeed())
	})

})
