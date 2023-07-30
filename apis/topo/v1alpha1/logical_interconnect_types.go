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

type LogicalInterconnectSpec struct {
	Links uint16 `json:"links,omitempty" yaml:"links,omitempty"`
	// lag, vesi
	Type *string `json:"type,omitempty" yaml:"type,omitempty"`
	// lacp
	Lacp *bool `json:"lacp,omitempty" yaml:"lacp,omitempty"`

	// Endpoints defines exactly 2 endpoints, the first entry is the local endpoint, the 2nd entry
	// is the remote endpoint
	// +kubebuilder:validation:MaxItems:=2
	// +kubebuilder:validation:MinItems:=2
	Endpoints []LogicalInterconnectEndpoint `json:"endpoints" yaml:"endpoints"`

	// UserDefinedLabels define metadata  associated to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

type LogicalInterconnectEndpoint struct {
	// Name is the name of logical endpoint. E.g. bond0
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// topologies define the topologies to which the endpoints of the
	// logical interconnect link belongs to
	// This allows multi-homing to different topologies
	Topologies []string `json:"topologies" yaml:"topologies"`

	// we need to find a way to indicate selecting endpoints from multiple
	// topologies or multiple nodes within a topologies
	// +kubebuilder:validation:Optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	// SelectorPolicy defines the policy used to select the endpoints
	// +kubebuilder:validation:Optional
	SelectorPolicy *SelectorPolicy `json:"selectorPolicy,omitempty" yaml:"selectorPolicy,omitempty"`
}

type SelectorPolicy struct {
	// NodeDiversity is a selection policy to select endpoints that are node diverse
	// NodeDiversity defines the amount of different nodes to be used when
	// selecting endpoints
	NodeDiversity *uint16 `json:"nodeDiversity,omitempty" yaml:"nodeDiversity,omitempty"`
	// TopologyDiversity is a selection policy to select endpoints in diverse topologies
	// TopologyDiversity *bool `json:"topologyDiversity,omitempty" yaml:"topologyDiversity,omitempty"`
}

// LogicalInterconnectStatus defines the observed state of LogicalInterconnect
type LogicalInterconnectStatus struct {
	// ConditionedStatus provides the status of the Interconnect using conditions
	// 1 conditions is used:
	// - a condition for the ready status
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={nephio,inv}
// Interconnect is the Schema for the interconnect API
type LogicalInterconnect struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   LogicalInterconnectSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status LogicalInterconnectStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogicalInterconnectList contains a list of LogicalInterconnect
type LogicalInterconnectList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []LogicalInterconnect `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalInterconnect{}, &LogicalInterconnectList{})
}

var (
	LogicalInterconnectKind             = reflect.TypeOf(LogicalInterconnect{}).Name()
	LogicalInterconnectGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: LogicalInterconnectKind}.String()
	LogicalInterconnectKindAPIVersion   = LogicalInterconnectKind + "." + GroupVersion.String()
	LogicalInterconnectGroupVersionKind = GroupVersion.WithKind(LogicalInterconnectKind)
	LogicalInterconnectKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    LogicalInterconnectKind,
	})
)
