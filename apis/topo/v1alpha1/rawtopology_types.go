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

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RawTopologySpec defines the desired state of RawTopology
type RawTopologySpec struct {
	//Defaults *NodeProperties `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	// Kinds map[string]NodeProperties `json:"kinds,omitempty" yaml:"kinds,omitempty"`
	Nodes map[string]invv1alpha1.NodeSpec `json:"nodes" yaml:"nodes"`
	Links []InterconnectLink              `json:"links" yaml:"links"`

	// UserDefinedLabels define metadata  associated to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`

	// Location provider the location information where this resource is located
	Location *invv1alpha1.Location `json:"location,omitempty" yaml:"location,omitempty"`
}

// RawTopologyStatus defines the observed state of RawTopology
type RawTopologyStatus struct {
	// ConditionedStatus provides the status of the RawTopology using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={nephio,inv}
// RawTopology is the Schema for the rawTopology API
type RawTopology struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   RawTopologySpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status RawTopologyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RawTopologyList contains a list of RawTopologys
type RawTopologyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []RawTopology `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&RawTopology{}, &RawTopologyList{})
}

var (
	RawTopologyKind             = reflect.TypeOf(RawTopology{}).Name()
	RawTopologyGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: RawTopologyKind}.String()
	RawTopologyKindAPIVersion   = RawTopologyKind + "." + GroupVersion.String()
	RawTopologyGroupVersionKind = GroupVersion.WithKind(RawTopologyKind)
	RawTopologyKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    RawTopologyKind,
	})
)
