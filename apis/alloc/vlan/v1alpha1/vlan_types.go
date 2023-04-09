/*
Copyright 2022 Nokia.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VLANSpec defines the desired state of VLAN
type VLANSpec struct {
	// VlanID defines a specific vlan id
	VlanID uint16 `json:"vlanID,omitempty" yaml:"vlanID,omitempty"`
	// VLANRange defines a range of vlans
	VLANRange string `json:"range,omitempty" yaml:"range,omitempty"`
	// Labels define metadata to the object (aka. user defined labels). They are part of the spec since the allocation
	// selector will use these labels for allocation more specific prefixes/addresses within this prefix
	// As such we distinguish clearly between the metadata labels and the user defined labels in the spec
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// VLANStatus defines the observed state of VLAN
type VLANStatus struct {
	// ConditionedStatus provides the status of the VLAN allocation using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	allocv1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	// AllocatedVlan identifies the vlan that was allocated by the VLAN backend
	AllocatedVlanID uint16 `json:"vlanID,omitempty" yaml:"vlanID,omitempty"`
	// AllocatedVlan identifies the vlan range that was allocated by the VLAN backend
	AllocatedVlanRange string `json:"vlanRange,omitempty" yaml:"vlanRange,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.kind=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.kind=='Ready')].status"
// +kubebuilder:printcolumn:name="VLAN-REQ",type="string",JSONPath=".spec.vlanID"
// +kubebuilder:printcolumn:name="VLAN-ALLOC",type="string",JSONPath=".status.vlanID"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,alloc}
// VLANAllocation is the Schema for the vlan allocations API
type VLAN struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   VLANSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status VLANStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VLANList contains a list of VLAN
type VLANList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []VLANAllocation `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&VLAN{}, &VLAN{})
}

var (
	VLANKind             = reflect.TypeOf(VLAN{}).Name()
	VLANGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: VLANKind}.String()
	VLANKindAPIVersion   = VLANKind + "." + GroupVersion.String()
	VLANGroupVersionKind = GroupVersion.WithKind(VLANKind)
	VLANKindGVKString    = meta.GVKToString(&schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    VLANKind,
	})
)
