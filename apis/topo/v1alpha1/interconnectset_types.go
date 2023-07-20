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

// InterconnectSetSpec defines the desired state of InterconnectSet
type InterconnectSetSpec struct {
	// Number of desired interconnects. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" yaml:"replicas,omitempty"`

	// Label selector for interconnects. Existing ReplicaSets whose pods are
	// selected by this will be the ones affected by this deployment.
	// It must match the pod template's labels.
	Selector *metav1.LabelSelector `json:"selector" protobuf:"bytes,2,opt,name=selector"`

	// Template describes the interconnects that will be created.
	Template InterconnectTemplateSpec `json:"template" yaml:"template"`
}

// InterconnectTemplateSpec describes the data a interconnect should have when created from a template
type InterconnectTemplateSpec struct {
	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the interconnect.
	// +optional
	Spec InterconnectSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// InterconnectSetStatus defines the observed state of Interconnect
type InterconnectSetStatus struct {
	// ConditionedStatus provides the status of the InterconnectSet using conditions
	// 1 conditions is used:
	// - a condition for the ready status
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={nephio,inv}
// Interconnect is the Schema for the interconnect API
type InterconnectSet struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   InterconnectSetSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status InterconnectSetStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InterconnectSetList contains a list of Interconnect sets
type InterconnectSetList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []InterconnectSet `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&Interconnect{}, &InterconnectList{})
}

var (
	InterconnectSetKind             = reflect.TypeOf(InterconnectSet{}).Name()
	InterconnectSetGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: InterconnectSetKind}.String()
	InterconnectSetKindAPIVersion   = InterconnectSetKind + "." + GroupVersion.String()
	InterconnectSetGroupVersionKind = GroupVersion.WithKind(InterconnectSetKind)
	InterconnectSetKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    InterconnectSetKind,
	})
)
