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

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IPAllocationSpec defines the desired state of IPAllocation
type IPAllocationSpec struct {
	// PrefixKind defines the kind of prefix we want to allocate
	// network kind is used for physical, virtual nics on a device
	// loopback kind is used for loopback interfaces
	// pool kind is used for pools for dhcp/radius/bng/upf/etc
	// aggregate kind is used for allocating an aggregate prefix
	// +kubebuilder:validation:Enum=`network`;`loopback`;`pool`;`aggregate`
	// +kubebuilder:default=network
	PrefixKind PrefixKind `json:"kind" yaml:"kind"`
	// NetworkInstance defines the networkInstance context used to allocate this prefix
	// Name and optionally Namespace is used here
	NetworkInstance *corev1.ObjectReference `json:"networkInstance" yaml:"networkInstance"`
	// AddressFamily defines the address family this prefix
	// +kubebuilder:validation:Enum=`ipv4`;`ipv6`
	AddressFamily iputil.AddressFamily `json:"addressFamily,omitempty" yaml:"addressFamily,omitempty"`
	// Prefix allows the client to define the prefix they want to be allocated
	// Used for specific prefix allocation or used as a hint for a dynamic prefix allocation in case of restart
	// +kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])/(([0-9])|([1-2][0-9])|(3[0-2]))|((:|[0-9a-fA-F]{0,4}):)([0-9a-fA-F]{0,4}:){0,5}((([0-9a-fA-F]{0,4}:)?(:|[0-9a-fA-F]{0,4}))|(((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])))(/(([0-9])|([0-9]{2})|(1[0-1][0-9])|(12[0-8])))`
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	// PrefixLength allows to client to indicate the prefixLength it wants for the allocation
	// Used typically to indicate the size a certain prefix
	// If not supplied we use assume /32 for ipv4 and /128 for ipv6
	PrefixLength uint8 `json:"prefixLength,omitempty" yaml:"prefixLength,omitempty"`
	// index defines the index in order for the backend to allocate a dedicated index in the prefix.
	// Used for deterministic prefix allocation
	Index uint32 `json:"index,omitempty" yaml:"index,omitempty"`
	// Selector defines the selector criterias by whihc this prefix should be allocated
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	// Labels define metadata to the object (aka. user defined labels). They are part of the spec since the allocation
	// selector will use these labels for allocation more specific prefixes/addresses within this prefix
	// As such we distinguish clearly between the metadata labels and the user defined labels in the spec
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	// CreatePrefix defines if this prefix must be created. Allows to specify a
	CreatePrefix bool `json:"createPrefix,omitempty" yaml:"createPrefix,omitempty"`
}

// IPAllocationStatus defines the observed state of IPAllocation
type IPAllocationStatus struct {
	// ConditionedStatus provides the status of the VLAN allocation using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	allocv1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
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
