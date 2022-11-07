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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IPAllocationSpec defines the desired state of IPAllocation
type IPAllocationSpec struct {
	// +kubebuilder:validation:Enum=`network`;`loopback`;`pool`;`aggregate`
	// +kubebuilder:default=network
	PrefixKind string `json:"kind"`
	// +kubebuilder:validation:Enum=`ipv4`;`ipv6`
	AddressFamily string `json:"addressFamily,omitempty"`
	// Prefix allows the client to indicate the prefix that was already allocated and validate if the allocation is still consistent
	// +kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])/(([0-9])|([1-2][0-9])|(3[0-2]))|((:|[0-9a-fA-F]{0,4}):)([0-9a-fA-F]{0,4}:){0,5}((([0-9a-fA-F]{0,4}:)?(:|[0-9a-fA-F]{0,4}))|(((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])))(/(([0-9])|([0-9]{2})|(1[0-1][0-9])|(12[0-8])))`
	Prefix string `json:"prefix,omitempty"`
	// PrefixLength allows to client to indicate the prefixLength he wants for the allocation
	PrefixLength uint8 `json:"prefixLength,omitempty"`
	// Label selector for selecting the context from which the IP prefix/address gets allocated
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// IPAllocationStatus defines the observed state of IPAllocation
type IPAllocationStatus struct {
	ConditionedStatus `json:",inline"`
	// AllocatedPrefix identifies the prefix that was allocated by the IPAM system
	AllocatedPrefix string `json:"prefix,omitempty"`
	// Gateway identifies the gatway IP for the network
	Gateway string `json:"gateway,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.kind=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.kind=='Ready')].status"
// +kubebuilder:printcolumn:name="KIND",type="string",JSONPath=".spec.kind"
// +kubebuilder:printcolumn:name="AF",type="string",JSONPath=".spec.addressFamily"
// +kubebuilder:printcolumn:name="PREFIXLENGTH",type="string",JSONPath=".spec.prefixLength"
// +kubebuilder:printcolumn:name="PREFIX-REQ",type="string",JSONPath=".spec.prefix"
// +kubebuilder:printcolumn:name="PREFIX-ALLOC",type="string",JSONPath=".status.prefix"
// +kubebuilder:printcolumn:name="GATEWAY",type="string",JSONPath=".status.gateway"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,ipam}
// IPAllocation is the Schema for the ipallocations API
type IPAllocation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPAllocationSpec   `json:"spec,omitempty"`
	Status IPAllocationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IPAllocationList contains a list of IPAllocation
type IPAllocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPAllocation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPAllocation{}, &IPAllocationList{})
}

var (
	IPAllocationKind             = reflect.TypeOf(IPAllocation{}).Name()
	IPAllocationGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: IPAllocationKind}.String()
	IPAllocationKindAPIVersion   = IPAllocationKind + "." + GroupVersion.String()
	IPAllocationGroupVersionKind = GroupVersion.WithKind(IPAllocationKind)
)
