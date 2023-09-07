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

// TopologySpec defines the desired state of a Topology
type TopologySpec struct {
}

// TopologyStatus defines the observed state of a Topology
type TopologyStatus struct {
	// ConditionedStatus provides the status of the Topology using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={nephio,inv},scope=Cluster
// Topology is the Schema for the topology API
type Topology struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   TopologySpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status TopologyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TopologyList contains a list of Topologies
type TopologyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []Topology `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&Topology{}, &TopologyList{})
}

var (
	TopologyKind             = reflect.TypeOf(Topology{}).Name()
	TopologyGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: TopologyKind}.String()
	TopologyKindAPIVersion   = TopologyKind + "." + GroupVersion.String()
	TopologyGroupVersionKind = GroupVersion.WithKind(TopologyKind)
	TopologyKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    TopologyKind,
	})
)
