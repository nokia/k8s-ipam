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

// EndpointSpec defines the desired state of Endpoint
type EndpointSpec struct {
	EndpointProperties `json:",inline" yaml:",inline"`
	Provider           `json:",inline" yaml:",inline"`
}

type EndpointProperties struct {
	InterfaceName string `json:"interfaceName" yaml:"interfaceName"`
	NodeName      string `json:"nodeName" yaml:"nodeName"`
	// LacpFallback defines if the link is part of a lag
	// mutually exclusive with Lag parameter
	// +optional
	LacpFallback *bool `json:"lacpFallback,omitempty" yaml:"lacpFallback,omitempty"`

	// MultiHoming defines if the endpoint is multi-homed
	// +optional
	MultiHoming *bool `json:"multiHoming,omitempty" yaml:"multiHoming,omitempty"`

	// MultiHomingName defines the name of the multi-homing
	// +optional
	MultiHomingName *string `json:"multiHomingName,omitempty" yaml:"multiHomingName,omitempty"`

	// UserDefinedLabels define metadata  associated to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

// EndpointStatus defines the observed state of Endpoint
type EndpointStatus struct {
	// ConditionedStatus provides the status of the Endpoint using conditions
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
// Endpoint is the Schema for the vlan API
type Endpoint struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   EndpointSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status EndpointStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EndpointList contains a list of Endpoints
type EndpointList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []Endpoint `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&Endpoint{}, &EndpointList{})
}

var (
	EndpointKind             = reflect.TypeOf(Endpoint{}).Name()
	EndpointGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: EndpointKind}.String()
	EndpointKindAPIVersion   = EndpointKind + "." + GroupVersion.String()
	EndpointGroupVersionKind = GroupVersion.WithKind(EndpointKind)
	EndpointKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    EndpointKind,
	})
)
