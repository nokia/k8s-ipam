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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VXLANIndexSpec defines the desired state of VXLANIndex
type VXLANIndexSpec struct {
	// Offset defines the offset where the vxlan index starts to claim vxlan IDs from
	Offset     uint32 `json:"offset" yaml:"offset"`
	// MaxEntryID defines the max vxlan entry id this index will claim
	MaxEntryID uint32 `json:"maxEntryID" yaml:"maxEntryID"`
	// UserDefinedLabels define metadata to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

// VXLANIndexStatus defines the observed state of VXLANIndex
type VXLANIndexStatus struct {
	// ConditionedStatus provides the status of the VXLAN Index using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,resource}
// VXLANIndex is the Schema for the vxlan database API
type VXLANIndex struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   VXLANIndexSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status VXLANIndexStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VXLANIndexList contains a list of VXLANIndices
type VXLANIndexList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []VXLANIndex `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&VXLANIndex{}, &VXLANIndexList{})
}

var (
	VXLANIndexKind             = reflect.TypeOf(VXLANIndex{}).Name()
	VXLANIndexGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: VXLANIndexKind}.String()
	VXLANIndexKindAPIVersion   = VXLANIndexKind + "." + GroupVersion.String()
	VXLANIndexGroupVersionKind = GroupVersion.WithKind(VXLANIndexKind)
	VXLANIndexKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    VXLANIndexKind,
	})
)
