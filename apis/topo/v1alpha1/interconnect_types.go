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

/*
import (
	"reflect"

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// InterconnectSpec defines the desired state of Interconnect
type InterconnectSpec struct {
	// Links define the links part of the interconnect
	Links []InterconnectLink `json:"links" yaml:"links"`
	// topologies define the topologies used to interconnect
	// can either be specified globally (here) or at the endpoint level of the
	// interconnectLink. When specified globally (here) it can only have 2 topologies
	// the local topologies will be
	// +kubebuilder:validation:MaxItems:=2
	// +kubebuilder:validation:MinItems:=2
	Topologies []string `json:"topologies,omitempty" yaml:"topologies,omitempty"`
}

type InterconnectLink struct {
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// LogicalLinkId defines the logical link id to which this logical link belongs
	// when using abstract interconnect links this is derived from the link index
	LogicalLinkId *int `json:"logicalLinkId,omitempty" yaml:"logicalLinkId,omitempty"`
	// Links define the physical links in an interconnect link
	// this means this is an abstract interconnect link
	Links *uint32 `json:"links,omitempty" yaml:"links,omitempty"`
	// lag, vesi
	Type *string `json:"type,omitempty" yaml:"type,omitempty"`

	Lacp *bool `json:"lacp,omitempty" yaml:"lacp,omitempty"`
	// Endpoints defines exactly 2 endpoints, the first entry is the local endpoint, the 2nd entry
	// is the remote endpoint
	// +kubebuilder:validation:MaxItems:=2
	// +kubebuilder:validation:MinItems:=2
	Endpoints []InterconnectLinkEndpoint `json:"endpoints" yaml:"endpoints"`

	// UserDefinedLabels define metadata  associated to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

// InterconnectLinkEndpoint
type InterconnectLinkEndpoint struct {
	// topology defines the topology to which this endpoint belongs
	Topology *string `json:"topology,omitempty" yaml:"topology,omitempty"`
	// logicalEndpointName
	LogicalEndpointName *string `json:"logicalEndpointName,omitempty" yaml:"logicalEndpointName,omitempty"`

	// InterfaceName defines the name of the interface on which this interconnect originates/terminates
	InterfaceName *string `json:"interfaceName,omitempty" yaml:"interfaceName,omitempty"`
	// NodeName provide the name of the node on which this interconnect originates/terminates
	// NodeName allows for multi-homing if multiple endpoints of a InterconnectLink reside on
	// different nodes
	NodeName *string `json:"nodeName,omitempty" yaml:"nodeName,omitempty"`

	// +kubebuilder:validation:Optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`

	SelectorPolicy *SelectorPolicy `json:"selectorPolicy,omitempty" yaml:"selectorPolicy,omitempty"`
}


// InterconnectStatus defines the observed state of Interconnect
type InterconnectStatus struct {
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
type Interconnect struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   InterconnectSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status InterconnectStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InterconnectList contains a list of Interconnects
type InterconnectList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []Interconnect `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&Interconnect{}, &InterconnectList{})
}

var (
	InterconnectKind             = reflect.TypeOf(Interconnect{}).Name()
	InterconnectGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: InterconnectKind}.String()
	InterconnectKindAPIVersion   = InterconnectKind + "." + GroupVersion.String()
	InterconnectGroupVersionKind = GroupVersion.WithKind(InterconnectKind)
	InterconnectKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    InterconnectKind,
	})
)
*/
