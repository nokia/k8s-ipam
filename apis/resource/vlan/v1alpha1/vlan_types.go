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

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VLANSpec defines the desired state of VLAN
type VLANSpec struct {
	// VLANIndex defines the vlan index for the VLAN claim
	VLANIndex corev1.ObjectReference `json:"vlanDatabase" yaml:"vlanDatabase"`
	// VLANID defines the VLAN ID
	VLANID *uint16 `json:"vlanID,omitempty" yaml:"vlanID,omitempty"`
	// VLANRange defines a range of vlans
	VLANRange *string `json:"range,omitempty" yaml:"range,omitempty"`
	// UserDefinedLabels define metadata to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

// VLANStatus defines the observed state of VLAN
type VLANStatus struct {
	// ConditionedStatus provides the status of the VLAN claim using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	// VLANID defines the vlan ID, claimed through the VLAN backend
	VLANID *uint16 `json:"vlanID,omitempty" yaml:"vlanID,omitempty"`
	// VLANRange defines the vlan range, claimed through the VLAN backend
	VLANRange *string `json:"vlanRange,omitempty" yaml:"vlanRange,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="VLAN-REQ",type="string",JSONPath=".spec.vlanID"
// +kubebuilder:printcolumn:name="VLAN-ALLOC",type="string",JSONPath=".status.vlanID"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,resource}
// VLAN is the Schema for the vlan API
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
	Items           []VLAN `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&VLAN{}, &VLANList{})
}

var (
	VLANKind             = reflect.TypeOf(VLAN{}).Name()
	VLANGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: VLANKind}.String()
	VLANKindAPIVersion   = VLANKind + "." + GroupVersion.String()
	VLANGroupVersionKind = GroupVersion.WithKind(VLANKind)
	VLANKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    VLANKind,
	})
)
