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

package hooks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
	"github.com/cybozu-go/necotiator/pkg/constants"
)

// log is for logging in this package.
var resourcequotalog = logf.Log.WithName("resourcequota-resource")

type resourceQuotaValidator struct {
	client         client.Client
	namespace      string
	serviceAccount string
}

func SetupResourceQuotaWebhookWithManager(mgr ctrl.Manager, ns, sa string) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.ResourceQuota{}).
		WithValidator(&resourceQuotaValidator{mgr.GetClient(), ns, sa}).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate--v1-resourcequota,mutating=false,failurePolicy=fail,sideEffects=None,groups=core,resources=resourcequotas,verbs=create;update,versions=v1,name=vresourcequota.kb.io,admissionReviewVersions=v1

var _ admission.CustomValidator = &resourceQuotaValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *resourceQuotaValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	resourcequotalog.Info("validate create")

	if rq, ok := obj.(*corev1.ResourceQuota); ok {
		return r.validate(ctx, rq)
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *resourceQuotaValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	resourcequotalog.Info("validate update")

	rq, ok := newObj.(*corev1.ResourceQuota)
	if !ok {
		return fmt.Errorf("unknown newObj type %T", newObj)
	}
	old, ok := oldObj.(*corev1.ResourceQuota)
	if !ok {
		return fmt.Errorf("unknown oldObj type %T", oldObj)
	}

	if err := r.validateLabelChange(ctx, old, rq); err != nil {
		return err
	}

	return r.validate(ctx, rq)
}

func (r *resourceQuotaValidator) validateLabelChange(ctx context.Context, oldObj, newObj *corev1.ResourceQuota) error {
	request, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}
	if request.UserInfo.Username == fmt.Sprintf("system:serviceaccount:%s:%s", r.namespace, r.serviceAccount) {
		return nil
	}

	if oldObj.Labels[constants.LabelTenant] != newObj.Labels[constants.LabelTenant] {
		err := apierrors.NewInvalid(
			schema.GroupKind{Group: corev1.GroupName, Kind: "ResourceQuota"},
			newObj.Name,
			field.ErrorList{field.Forbidden(
				field.NewPath("metadata", "labels", constants.LabelTenant),
				"tenant labels is immutable",
			)})
		log.FromContext(ctx).Error(err, "validation error")
		return err
	}
	return nil
}

func (v *resourceQuotaValidator) validate(ctx context.Context, rq *corev1.ResourceQuota) error {
	logger := log.FromContext(ctx)

	tenantName, ok := rq.Labels[constants.LabelTenant]
	if !ok {
		return nil
	}

	var quota necotiatorv1beta1.TenantResourceQuota
	err := v.client.Get(ctx, client.ObjectKey{Name: tenantName}, &quota)
	if err != nil {
		return err
	}

	allocated := quota.Status.Allocated

	var errs field.ErrorList
	for resourceName, requested := range rq.Spec.Hard {
		allocatedResource := allocated[resourceName]
		limit, ok := quota.Spec.Hard[resourceName]
		if !ok {
			continue
		}

		if requested.Cmp(resource.MustParse("0")) == 0 {
			continue
		}

		newTotal := allocatedResource.Total
		if oldAllocated, ok := allocatedResource.Namespaces[rq.GetNamespace()]; ok {
			if requested.Cmp(oldAllocated) <= 0 {
				continue
			}
			newTotal.Sub(oldAllocated)
		}
		newTotal.Add(requested)

		if newTotal.Cmp(limit) > 0 {
			errs = append(errs, field.Forbidden(
				field.NewPath("spec", "hard", string(resourceName)),
				fmt.Sprintf(
					"exceeded tenant quota: %s, requested: %s=%s, total: %s=%s, limited: %s=%s",
					tenantName,
					resourceName, requested.String(),
					resourceName, newTotal.String(),
					resourceName, limit.String(),
				),
			))
		}
	}
	for resourceName := range quota.Spec.Hard {
		if _, ok := rq.Spec.Hard[resourceName]; !ok {
			errs = append(errs, field.Required(
				field.NewPath("spec", "hard", string(resourceName)),
				fmt.Sprintf(
					"required %s by tenant resource quota: %s",
					resourceName, tenantName,
				),
			))
		}
	}

	if len(errs) > 0 {
		err := apierrors.NewInvalid(schema.GroupKind{Group: corev1.GroupName, Kind: "ResourceQuota"}, rq.Name, errs)
		logger.Error(err, "validation error")
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *resourceQuotaValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	resourcequotalog.Info("validate delete")

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
