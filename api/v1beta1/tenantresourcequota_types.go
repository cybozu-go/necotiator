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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TenantResourceQuotaSpec defines the desired state of TenantResourceQuota
type TenantResourceQuotaSpec struct {
	// Hard is the set of desired hard limits for each tenant.
	// +optional
	Hard corev1.ResourceList `json:"hard,omitempty"`

	// NamespaceSelector is used to select namespaces by label.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// ResourceUsage is aggregated usages of the resource.
type ResourceUsage struct {
	// Total is total observed usage of the resource.
	// +optional
	Total resource.Quantity `json:"total,omitempty"`

	// Namespaces is observed usage of the resource per namespace.
	// +optional
	Namespaces map[string]resource.Quantity `json:"namespaces,omitempty"`
}

// TenantResourceQuotaStatus defines the observed state of TenantResourceQuota
type TenantResourceQuotaStatus struct {
	// Allocated is the current observed allocated resources to namespaces in the tenant.
	// +optional
	Allocated map[corev1.ResourceName]ResourceUsage `json:"allocated,omitempty"`

	// Used is the current observed usage of the resource in the tenant.
	// +optional
	Used map[corev1.ResourceName]ResourceUsage `json:"used,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// TenantResourceQuota is the Schema for the tenantresourcequota API
type TenantResourceQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantResourceQuotaSpec   `json:"spec,omitempty"`
	Status TenantResourceQuotaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantResourceQuotaList contains a list of TenantResourceQuota
type TenantResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantResourceQuota `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TenantResourceQuota{}, &TenantResourceQuotaList{})
}
