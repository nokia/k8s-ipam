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

// VXLANClaimSpec defines the desired state of VXLANClaim
type VXLANClaimSpec struct {
	// VXLANIndex defines the vxlan index for the VXLAN Claim
	VXLANIndex corev1.ObjectReference `json:"vxlanIndex" yaml:"vxlanIndex"`
	// ClaimLabels define the user defined labels and selector labels used
	// in resource claim
	resourcev1alpha1.ClaimLabels `json:",inline" yaml:",inline"`
}

// VXLANClaimStatus defines the observed state of VXLANClaim
type VXLANClaimStatus struct {
	// ConditionedStatus provides the status of the VXLAN clain using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	// VXLANID defines the vxlan ID, claimed through the VXLAN backend
	VXLANID *uint32 `json:"vxlanID,omitempty" yaml:"vxlanID,omitempty"`
	// ExpiryTime indicated when the claim expires
	// +kubebuilder:validation:Optional
	ExpiryTime string `json:"expiryTime,omitempty" yaml:"expiryTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="VXLAN-REQ",type="string",JSONPath=".spec.vxlanID"
// +kubebuilder:printcolumn:name="VXLAN-ALLOC",type="string",JSONPath=".status.vxlanID"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,resource}
// VXLANClaim is the Schema for the vxlan claim API
type VXLANClaim struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   VXLANClaimSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status VXLANClaimStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VXLANClaimList contains a list of VXLANClaims
type VXLANClaimList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []VXLANClaim `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&VXLANClaim{}, &VXLANClaimList{})
}

var (
	VXLANClaimKind             = reflect.TypeOf(VXLANClaim{}).Name()
	VXLANClaimGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: VXLANClaimKind}.String()
	VXLANClaimKindAPIVersion   = VXLANClaimKind + "." + GroupVersion.String()
	VXLANClaimGroupVersionKind = GroupVersion.WithKind(VXLANClaimKind)
	VXLANClaimKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    VXLANClaimKind,
	})
)
