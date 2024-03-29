/*
Copyright 2022 Nokia.

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

// NetworkInstanceSpec defines the desired state of NetworkInstance
type NetworkInstanceSpec struct {
	// Prefixes define the aggregate prefixes for the network instance
	// A Network instance needs at least 1 prefix to be defined to become operational
	Prefixes []Prefix `json:"prefixes" yaml:"prefixes"`
}

type Prefix struct {
	// Prefix defines the ip cidr in prefix notation.
	// +kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])/(([0-9])|([1-2][0-9])|(3[0-2]))|((:|[0-9a-fA-F]{0,4}):)([0-9a-fA-F]{0,4}:){0,5}((([0-9a-fA-F]{0,4}:)?(:|[0-9a-fA-F]{0,4}))|(((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])))(/(([0-9])|([0-9]{2})|(1[0-1][0-9])|(12[0-8])))`
	Prefix string `json:"prefix" yaml:"prefix"`
	// UserDefinedLabels define metadata to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
}

// NetworkInstanceStatus defines the observed state of NetworkInstance
type NetworkInstanceStatus struct {
	// ConditionedStatus provides the status of the NI using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	// Prefixes defines the prefixes, claimed through the IPAM backend
	Prefixes []Prefix `json:"prefixes,omitempty" yaml:"prefixes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="NETWORK-INSTANCE",type="string",JSONPath=".metadata.name"
// +kubebuilder:printcolumn:name="PREFIX0",type="string",JSONPath=".spec.prefixes[0].prefix"
// +kubebuilder:printcolumn:name="PREFIX1",type="string",JSONPath=".spec.prefixes[1].prefix"
// +kubebuilder:printcolumn:name="PREFIX2",type="string",JSONPath=".spec.prefixes[2].prefix"
// +kubebuilder:printcolumn:name="PREFIX3",type="string",JSONPath=".spec.prefixes[3].prefix"
// +kubebuilder:printcolumn:name="PREFIX4",type="string",JSONPath=".spec.prefixes[4].prefix"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,resource}
// NetworkInstance is the Schema for the networkinstances API
type NetworkInstance struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   NetworkInstanceSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status NetworkInstanceStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkInstanceList contains a list of NetworkInstance
type NetworkInstanceList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []NetworkInstance `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkInstance{}, &NetworkInstanceList{})
}

var (
	NetworkInstanceKind             = reflect.TypeOf(NetworkInstance{}).Name()
	NetworkInstancegroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: NetworkInstanceKind}.String()
	NetworkInstanceAPIVersion       = NetworkInstanceKind + "." + GroupVersion.String()
	NetworkInstanceGroupVersionKind = GroupVersion.WithKind(NetworkInstanceKind)
	NetworkInstanceKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    NetworkInstanceKind,
	})
)
