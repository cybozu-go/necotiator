/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
)

// TenantResourceQuotaReconciler reconciles a TenantResourceQuota object
type TenantResourceQuotaReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=necotiator.cybozu.io,resources=tenantresourcequotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=necotiator.cybozu.io,resources=tenantresourcequotas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=necotiator.cybozu.io,resources=tenantresourcequotas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TenantResourceQuota object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *TenantResourceQuotaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var quota necotiatorv1beta1.TenantResourceQuota
	err := r.Get(ctx, req.NamespacedName, &quota)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var namespaces corev1.NamespaceList

	selector, err := metav1.LabelSelectorAsSelector(quota.Spec.NamespaceSelector)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.List(ctx, &namespaces, &client.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, ns := range namespaces.Items {
		err := r.reconcileResourceQuota(ctx, &quota, &ns)
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Reconciled", "namespace", ns.GetName())
	}

	err = r.updateStatus(ctx, &quota, &namespaces)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling", "namespaces", namespaces)

	return ctrl.Result{}, nil
}

func (r *TenantResourceQuotaReconciler) updateStatus(ctx context.Context, tenantQuota *necotiatorv1beta1.TenantResourceQuota, namespaceList *corev1.NamespaceList) error {
	allocated := make(map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage)
	used := make(map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage)

	for _, namespace := range namespaceList.Items {
		var quota corev1.ResourceQuota
		err := r.Get(ctx, client.ObjectKey{Namespace: namespace.Name, Name: "default"}, &quota)
		if err != nil {
			return err
		}
		if quota.Labels["necotiator.cybozu.io/tenant"] != tenantQuota.Name {
			log.FromContext(ctx).Error(nil, "Ignore unmatched label namespace", "namespace", namespace.Name)
			r.Recorder.Event(tenantQuota, corev1.EventTypeWarning, "IgnoredNamespace", fmt.Sprintf("Ignored unmatched label namespace: %s", namespace.Name))
			continue
		}

		addResourceUsage(allocated, quota.Status.Hard, namespace.Name)
		addResourceUsage(used, quota.Status.Used, namespace.Name)
	}

	old := tenantQuota.DeepCopy()

	tenantQuota.Status.Allocated = allocated
	tenantQuota.Status.Used = used

	if equality.Semantic.DeepEqual(old.Status, tenantQuota.Status) {
		return nil
	}

	log.FromContext(ctx).Info("Updating status")
	err := r.Status().Update(ctx, tenantQuota)
	if err != nil {
		return err
	}

	return nil
}

func addResourceUsage(usageMap map[corev1.ResourceName]necotiatorv1beta1.ResourceUsage, resourceList corev1.ResourceList, namespaceName string) {
	for resourceName, hard := range resourceList {
		if usage, ok := usageMap[resourceName]; !ok {
			usageMap[resourceName] = necotiatorv1beta1.ResourceUsage{
				Total: hard,
				Namespaces: map[string]resource.Quantity{
					namespaceName: hard,
				}}
		} else {
			usage.Total.Add(hard)
			usage.Namespaces[namespaceName] = hard
			usageMap[resourceName] = usage
		}
	}
}

func (r *TenantResourceQuotaReconciler) reconcileResourceQuota(ctx context.Context, tenantQuota *necotiatorv1beta1.TenantResourceQuota, ns *corev1.Namespace) error {
	quota := corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.GetName(),
			Name:      "default",
			Labels: map[string]string{
				"app.kubernetes.io/created-by": "necotiator",
				"necotiator.cybozu.io/tenant":  tenantQuota.GetName(),
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: make(corev1.ResourceList, len(tenantQuota.Spec.Hard)),
		},
	}
	for res := range tenantQuota.Spec.Hard {
		quota.Spec.Hard[res] = resource.MustParse("0")
	}

	err := r.Create(ctx, &quota)
	if errors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantResourceQuotaReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	logger := log.FromContext(ctx)

	mapNamespace := func(o client.Object) []reconcile.Request {
		var quotas necotiatorv1beta1.TenantResourceQuotaList
		err := mgr.GetClient().List(ctx, &quotas)
		if err != nil {
			logger.Error(err, "watch namespace")
			return nil
		}

		var reqs []reconcile.Request
		for _, quota := range quotas.Items {
			selector, err := metav1.LabelSelectorAsSelector(quota.Spec.NamespaceSelector)
			if err != nil {
				logger.Error(err, "parsing tenant resource quota selector")
				continue
			}
			if selector.Matches(labels.Set(o.GetLabels())) {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: quota.GetName(),
					},
				})
			}
		}

		return reqs
	}
	mapResourceQuota := func(o client.Object) []reconcile.Request {
		tenant := o.GetLabels()["necotiator.cybozu.io/tenant"]
		if tenant == "" {
			return nil
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name: tenant,
				},
			},
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&necotiatorv1beta1.TenantResourceQuota{}).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, handler.EnqueueRequestsFromMapFunc(mapNamespace)).
		Watches(&source.Kind{Type: &corev1.ResourceQuota{}}, handler.EnqueueRequestsFromMapFunc(mapResourceQuota)).
		Complete(r)
}
