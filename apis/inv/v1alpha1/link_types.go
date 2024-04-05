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

// LinkSpec defines the desired state of Link
type LinkSpec struct {
	// Endpoints define the node + interface endpoints associated with this link
	// +kubebuilder:validation:MaxItems:=2
	// +kubebuilder:validation:MinItems:=2
	Endpoints []LinkEndpointSpec `json:"endpoints"`
	// UserDefinedLabels define metadata  associated to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	//resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

type LinkEndpointSpec struct {
	// topology defines the topology to which this endpoint belongs
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	Topology string `json:"topology" yaml:"topology"`
	// EndpointSpec defines the desired state of Endpoint
	EndpointSpec `json:",inline" yaml:",inline"`
}

// LinkStatus defines the observed state of Link
type LinkStatus struct {
	// ConditionedStatus provides the status of the Link using conditions
	// 2 conditions are used:
	// - a condition for the ready status
	// - a condition for the wire status if deployed
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="WIRED",type="string",JSONPath=".status.conditions[?(@.type=='Wired')].status"
// +kubebuilder:printcolumn:name="TOPOLOGY_0",type="string",JSONPath=".spec.endpoints[0].topology"
// +kubebuilder:printcolumn:name="NODE_NAME_0",type="string",JSONPath=".spec.endpoints[0].nodeName"
// +kubebuilder:printcolumn:name="IF_NAME_0",type="string",JSONPath=".spec.endpoints[0].interfaceName"
// +kubebuilder:printcolumn:name="TOPOLOGY_1",type="string",JSONPath=".spec.endpoints[1].topology"
// +kubebuilder:printcolumn:name="NODE_NAME_1",type="string",JSONPath=".spec.endpoints[1].nodeName"
// +kubebuilder:printcolumn:name="IF_NAME_1",type="string",JSONPath=".spec.endpoints[1].interfaceName"
// +kubebuilder:resource:categories={nephio,inv}
// Link is the Schema for the vlan API
type Link struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   LinkSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status LinkStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LinkList contains a list of Links
type LinkList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []Link `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&Link{}, &LinkList{})
}

var (
	LinkKind             = reflect.TypeOf(Link{}).Name()
	LinkGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: LinkKind}.String()
	LinkKindAPIVersion   = LinkKind + "." + GroupVersion.String()
	LinkGroupVersionKind = GroupVersion.WithKind(LinkKind)
	LinkKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    LinkKind,
	})
)
