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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
)

// log is for logging in this package.
var tenantresourcequotalog = logf.Log.WithName("tenantresourcequota-resource")

type tenantResourceQuotaMutator struct {
}

func SetupTenantResourceQuotaWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&necotiatorv1beta1.TenantResourceQuota{}).
		WithDefaulter(&tenantResourceQuotaMutator{}).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/mutate-necotiator-cybozu-io-v1beta1-tenantresourcequota,mutating=true,failurePolicy=fail,sideEffects=None,groups=necotiator.cybozu.io,resources=tenantresourcequotas,verbs=create;update,versions=v1beta1,name=mtenantresourcequota.kb.io,admissionReviewVersions=v1

var _ admission.CustomDefaulter = &tenantResourceQuotaMutator{}

// Default implements admission.CustomDefaulter so a webhook will be registered for the type
func (m *tenantResourceQuotaMutator) Default(ctx context.Context, obj runtime.Object) error {
	tenantresourcequotalog.Info("default")

	quota, ok := obj.(*necotiatorv1beta1.TenantResourceQuota)
	if !ok {
		return fmt.Errorf("unknown obj type: %T", obj)
	}

	if !controllerutil.ContainsFinalizer(quota, "necotiator.cybozu.io/finalizer") {
		logger := log.FromContext(ctx)
		logger.Info("add finalizer")
		controllerutil.AddFinalizer(quota, "necotiator.cybozu.io/finalizer")
	}

	return nil
}
