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
	// Kind defines the kind of prefix for the IP Allocation
	// - network kind is used for physical, virtual nics on a device
	// - loopback kind is used for loopback interfaces
	// - pool kind is used for pools for dhcp/radius/bng/upf/etc
	// - aggregate kind is used for allocating an aggregate prefix
	// +kubebuilder:validation:Enum=`network`;`loopback`;`pool`;`aggregate`
	// +kubebuilder:default=network
	Kind PrefixKind `json:"kind" yaml:"kind"`
	// NetworkInstance defines the networkInstance context for the IP allocation
	// Name and optionally Namespace is used here
	NetworkInstance corev1.ObjectReference `json:"networkInstance" yaml:"networkInstance"`
	// AddressFamily defines the address family for the IP allocation
	// +kubebuilder:validation:Enum=`ipv4`;`ipv6`
	// +kubebuilder:validation:Optional
	AddressFamily *iputil.AddressFamily `json:"addressFamily,omitempty" yaml:"addressFamily,omitempty"`
	// Prefix defines the prefix for the IP allocation
	// Used for specific prefix allocation or used as a hint for a dynamic prefix allocation in case of restart
	// +kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])/(([0-9])|([1-2][0-9])|(3[0-2]))|((:|[0-9a-fA-F]{0,4}):)([0-9a-fA-F]{0,4}:){0,5}((([0-9a-fA-F]{0,4}:)?(:|[0-9a-fA-F]{0,4}))|(((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])))(/(([0-9])|([0-9]{2})|(1[0-1][0-9])|(12[0-8])))`
	// +kubebuilder:validation:Optional
	Prefix *string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	// PrefixLength defines the prefix length for the IP Allocation
	// If not present we use assume /32 for ipv4 and /128 for ipv6 during allocation
	// +kubebuilder:validation:Optional
	PrefixLength *uint8 `json:"prefixLength,omitempty" yaml:"prefixLength,omitempty"`
	// Index defines the index of the IP Allocation, used to get a deterministic IP from a prefix
	// If not present we allocate a random prefix from a prefix
	// +kubebuilder:validation:Optional
	Index *uint32 `json:"index,omitempty" yaml:"index,omitempty"`
	// CreatePrefix defines if this prefix must be created. Only used for non address prefixes
	// e.g. non /32 ipv4 and non /128 ipv6 prefixes
	// +kubebuilder:validation:Optional
	CreatePrefix *bool `json:"createPrefix,omitempty" yaml:"createPrefix,omitempty"`
	// AllocationLabels define the user defined labels and selector labels used
	// in resource allocation
	allocv1alpha1.AllocationLabels `json:",inline" yaml:",inline"`
}

// IPAllocationStatus defines the observed state of IPAllocation
type IPAllocationStatus struct {
	// ConditionedStatus provides the status of the IP allocation using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	allocv1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	// Prefix defines the prefix, allocated by the IPAM backend
	// +kubebuilder:validation:Optional
	Prefix *string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	// Gateway defines the gateway IP for the allocated prefix
	// Gateway is only relevant for prefix kind = network
	// +kubebuilder:validation:Optional
	Gateway *string `json:"gateway,omitempty" yaml:"gateway,omitempty"`
	// ExpiryTime defines when the allocation expires
	// +kubebuilder:validation:Optional
	ExpiryTime *string `json:"expiryTime,omitempty" yaml:"expiryTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="NETWORK-INSTANCE",type="string",JSONPath=".spec.networkInstance"
// +kubebuilder:printcolumn:name="KIND",type="string",JSONPath=".spec.kind"
// +kubebuilder:printcolumn:name="AF",type="string",JSONPath=".spec.addressFamily"
// +kubebuilder:printcolumn:name="PREFIXLENGTH",type="string",JSONPath=".spec.prefixLength"
// +kubebuilder:printcolumn:name="PREFIX-REQ",type="string",JSONPath=".spec.prefix"
// +kubebuilder:printcolumn:name="PREFIX-ALLOC",type="string",JSONPath=".status.prefix"
// +kubebuilder:printcolumn:name="GATEWAY",type="string",JSONPath=".status.gateway"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,alloc}
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
	IPAllocationKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    IPAllocationKind,
	})
)
