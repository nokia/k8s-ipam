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

// LogicalEndpointSpec defines the desired state of LogicalEndpoint
type LogicalEndpointSpec struct {
	// MultiHoming defines if this logical endpoint is multi-homed
	//MultiHoming *bool `json:"multiHoming,omitempty" yaml:"multiHoming,omitempty"`
	// Name defines the logical endpoint name
	// can be single-homed or multi-homed
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// Lacp defines if the lag enabled LACP
	// +optional
	Lacp *bool `json:"lacp,omitempty" yaml:"lacp,omitempty"`
	// Endpoints define the endpoints that belong to the logical link
	// this can be more than 2
	Endpoints []EndpointSpec `json:"endpoints" yaml:"endpoints"`
	// UserDefinedLabels define metadata  associated to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

// LogicalEndpointStatus defines the observed state of LogicalEndpoint
type LogicalEndpointStatus struct {
	// ConditionedStatus provides the status of the LogicalEndpoint using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	// ESI defines the ethernet segment identifier of the logical link
	// if set this is a multi-homed logical endpoint
	// the ESI is a global unique identifier within the administrative domain/topology
	ESI *uint32 `json:"esi,omitempty" yaml:"esi,omitempty"`
	// LagId defines the lag id for the logical single-homed or multi-homed
	// endpoint
	LagId *uint32 `json:"lagId,omitempty" yaml:"lagId,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={nephio,inv}
// LogicalEndpoint is the Schema for the vlan API
type LogicalEndpoint struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   LogicalEndpointSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status LogicalEndpointStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogicalEndpointList contains a list of LogicalEndpoints
type LogicalEndpointList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []LogicalEndpoint `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalEndpoint{}, &LogicalEndpointList{})
}

var (
	LogicalEndpointKind             = reflect.TypeOf(LogicalEndpoint{}).Name()
	LogicalEndpointGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: LogicalEndpointKind}.String()
	LogicalEndpointKindAPIVersion   = LogicalEndpointKind + "." + GroupVersion.String()
	LogicalEndpointGroupVersionKind = GroupVersion.WithKind(LogicalEndpointKind)
	LogicalEndpointKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    LogicalEndpointKind,
	})
)
