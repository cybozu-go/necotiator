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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var resourcequotalog = logf.Log.WithName("resourcequota-resource")

type resourceQuotaValidator struct {
}

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.ResourceQuota{}).
		WithValidator(&resourceQuotaValidator{}).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-core-v1-resourcequota,mutating=false,failurePolicy=fail,sideEffects=None,groups=core,resources=resourcequota,verbs=create;update,versions=v1,name=vresourcequota.kb.io,admissionReviewVersions=v1

var _ admission.CustomValidator = &resourceQuotaValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *resourceQuotaValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	resourcequotalog.Info("validate create")

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *resourceQuotaValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	resourcequotalog.Info("validate update")

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *resourceQuotaValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	resourcequotalog.Info("validate delete")

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
