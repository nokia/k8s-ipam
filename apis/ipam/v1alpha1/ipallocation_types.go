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

	"github.com/nokia/k8s-ipam/internal/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IPAllocationSpec defines the desired state of IPAllocation
type IPAllocationSpec struct {
	// +kubebuilder:validation:Enum=`network`;`loopback`;`pool`;`aggregate`
	// +kubebuilder:default=network
	PrefixKind PrefixKind `json:"kind" yaml:"kind"`
	// NetworkInstance identifies the network instance the IP allocation is allocated from
	NetworkInstance string `json:"networkInstance" yaml:"networkInstance"`
	// +kubebuilder:validation:Enum=`ipv4`;`ipv6`
	//AddressFamily iputil.AddressFamily `json:"addressFamily,omitempty" yaml:"addressFamily,omitempty"`
	// Prefix allows the client to indicate the prefix that was already allocated and validate if the allocation is still consistent
	// +kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])/(([0-9])|([1-2][0-9])|(3[0-2]))|((:|[0-9a-fA-F]{0,4}):)([0-9a-fA-F]{0,4}:){0,5}((([0-9a-fA-F]{0,4}:)?(:|[0-9a-fA-F]{0,4}))|(((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])))(/(([0-9])|([0-9]{2})|(1[0-1][0-9])|(12[0-8])))`
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	// PrefixLength allows to client to indicate the prefixLength he wants for the allocation
	// used for prefixes, if not supplied we use eother /32 for ipv4 and /128 for ipv6
	PrefixLength uint8 `json:"prefixLength,omitempty" yaml:"prefixLength,omitempty"`
	// index is used for deterministic allocation
	Index uint32 `json:"index,omitempty" yaml:"index,omitempty"`
	// Label selector for selecting the context from which the IP prefix/address gets allocated
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	// Labels provide metadata to the prefix. They are part of the spec since the allocation
	// selector will use these labels for allocation more specific prefixes/addresses within this prefix
	// As such we distinguish clearly between the metadata labels and the labels used in the spec
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	// indicates a prefix has to be created which is not an address
	CreatePrefix bool `json:"createPrefix,omitempty" yaml:"createPrefix,omitempty"`
	// expiryTime indicated when the allocation expires
	ExpiryTime string `json:"expiryTime,omitempty" yaml:"expiryTime,omitempty"`
}

// IPAllocationStatus defines the observed state of IPAllocation
type IPAllocationStatus struct {
	ConditionedStatus `json:",inline" yaml:",inline"`
	// AllocatedPrefix identifies the prefix that was allocated by the IPAM system
	AllocatedPrefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	// Gateway identifies the gatway IP for the network
	Gateway string `json:"gateway,omitempty" yaml:"gateway,omitempty"`
	// expiryTime indicated when the allocation expires
	ExpiryTime string `json:"expiryTime,omitempty" yaml:"expiryTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.kind=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.kind=='Ready')].status"
// +kubebuilder:printcolumn:name="NETWORK-INSTANCE",type="string",JSONPath=".spec.networkInstance"
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
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   IPAllocationSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status IPAllocationStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IPAllocationList contains a list of IPAllocation
type IPAllocationList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []IPAllocation `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&IPAllocation{}, &IPAllocationList{})
}

var (
	IPAllocationKind             = reflect.TypeOf(IPAllocation{}).Name()
	IPAllocationGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: IPAllocationKind}.String()
	IPAllocationKindAPIVersion   = IPAllocationKind + "." + GroupVersion.String()
	IPAllocationGroupVersionKind = GroupVersion.WithKind(IPAllocationKind)
	IPAllocationKindGVKString    = meta.GVKToString(&schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    IPAllocationKind,
	})
)
