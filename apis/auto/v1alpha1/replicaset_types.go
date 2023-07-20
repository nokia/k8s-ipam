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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ReplicaSetSpec defines the desired state of ReplicaSet
type ReplicaSetSpec struct {
	// Number of desired replicas. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" yaml:"replicas,omitempty"`

	// Label selector for interconnects. Existing ReplicaSets whose pods are
	// selected by this will be the ones affected by this deployment.
	// It must match the pod template's labels.
	ObjectSelectors []ObjectSelector `json:"selectors,omitempty" yaml:"selectors,omitempty"`

	// Template is the embedded krm template
	//+kubebuilder:pruning:PreserveUnknownFields
	Template runtime.RawExtension `json:"template" yaml:"template"`
}

type ObjectSelector struct {
	SelectorVariable string `json:"selectorVariable" yaml:"selectorVariable"`

	metav1.LabelSelector `json:",inline" yaml:",inline"`
	// APIVersion of the target resources
	// +optional
	APIVersion *string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`

	// Kind of the target resources
	// +optional
	Kind *string `json:"kind,omitempty" yaml:"kind,omitempty"`

	// Name of the target resource
	// +optional
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`

	// Note: while namespace is not allowed; the namespace
	// must match the namespace of the ReplicaSet resource
}

// ReplicaSetStatus defines the observed state of ReplicaSet
type ReplicaSetStatus struct {
	// ConditionedStatus provides the status of the ReplicaSet using conditions
	// 1 conditions is used:
	// - a condition for the ready status
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={nephio,inv}
// ReplicaSet is the Schema for the ReplicaSet API
type ReplicaSet struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   ReplicaSetSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status ReplicaSetStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReplicaSetList contains a list of ReplicaSets
type ReplicaSetList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []ReplicaSet `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&ReplicaSet{}, &ReplicaSetList{})
}

var (
	ReplicaSetKind             = reflect.TypeOf(ReplicaSet{}).Name()
	ReplicaSetGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: ReplicaSetKind}.String()
	ReplicaSetKindAPIVersion   = ReplicaSetKind + "." + GroupVersion.String()
	ReplicaSetGroupVersionKind = GroupVersion.WithKind(ReplicaSetKind)
	ReplicaSetKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    ReplicaSetKind,
	})
)
