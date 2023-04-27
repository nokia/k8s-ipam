/*
Copyright 2023 The Nephio Authors.

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

package v1alpha1

import (
	"reflect"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VLANAllocationSpec defines the desired state of VLANAllocation
type VLANAllocationSpec struct {
	// VLANDatabase defines the vlan database contexts for the VLAN Allocation
	VLANDatabase corev1.ObjectReference `json:"vlanDatabase" yaml:"vlanDatabase"`
	// VLANID defines the vlan for the VLAN allocation
	VLANID *uint16 `json:"vlanID,omitempty" yaml:"vlanID,omitempty"`
	// VLANRange defines the vlan range for the VLAN allocation
	VLANRange *string `json:"range,omitempty" yaml:"range,omitempty"`
	// AllocationLabels define the user defined labels and selector labels used
	// in resource allocation
	allocv1alpha1.AllocationLabels `json:",inline" yaml:",inline"`
}

// VLANAllocationStatus defines the observed state of VLANAllocation
type VLANAllocationStatus struct {
	// ConditionedStatus provides the status of the VLAN allocation using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	allocv1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	// VLANID defines the vlan ID, allocated by the VLAN backend
	VLANID *uint16 `json:"vlanID,omitempty" yaml:"vlanID,omitempty"`
	// VLANRange defines the vlan range, allocated by the VLAN backend
	VLANRange *string `json:"vlanRange,omitempty" yaml:"vlanRange,omitempty"`
	// ExpiryTime indicated when the allocation expires
	// +kubebuilder:validation:Optional
	ExpiryTime string `json:"expiryTime,omitempty" yaml:"expiryTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="VLAN-REQ",type="string",JSONPath=".spec.vlanID"
// +kubebuilder:printcolumn:name="VLAN-ALLOC",type="string",JSONPath=".status.vlanID"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,alloc}
// VLANAllocation is the Schema for the vlan allocations API
type VLANAllocation struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   VLANAllocationSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status VLANAllocationStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VLANAllocationList contains a list of VLANAllocation
type VLANAllocationList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []VLANAllocation `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&VLANAllocation{}, &VLANAllocationList{})
}

var (
	VLANAllocationKind             = reflect.TypeOf(VLANAllocation{}).Name()
	VLANAllocationGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: VLANAllocationKind}.String()
	VLANAllocationKindAPIVersion   = VLANAllocationKind + "." + GroupVersion.String()
	VLANAllocationGroupVersionKind = GroupVersion.WithKind(VLANAllocationKind)
	VLANAllocationKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    VLANAllocationKind,
	})
)
