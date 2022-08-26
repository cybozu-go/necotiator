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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=necotiator.cybozu.io,resources=tenantresourcequotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=necotiator.cybozu.io,resources=tenantresourcequotas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=necotiator.cybozu.io,resources=tenantresourcequotas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

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

	logger.Info("Reconciling", "namespaces", namespaces)

	return ctrl.Result{}, nil
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&necotiatorv1beta1.TenantResourceQuota{}).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
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
		})).
		Complete(r)
}
