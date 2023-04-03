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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type VLANDBKind string

const (
	VLANDBKindESG    VLANDBKind = "esg"
	VLANDBKindDevice VLANDBKind = "device"
)

// VLANDatabaseSpec defines the desired state of VLANDatabase
type VLANDatabaseSpec struct {
	// VLANDBKind defines the kind of vlan database we want to create
	// esg kind is used for ethernet segment groups
	// device kind is used for devices: vlan per device or vlan per interface
	// +kubebuilder:validation:Enum=`esg`;`device`
	// +kubebuilder:default=esg
	VLANDBKind VLANDBKind `json:"kind" yaml:"kind"`
	// Labels define metadata to the object (aka. user defined labels). They are part of the spec since the allocation
	// selector will use these labels for allocation more specific prefixes/addresses within this prefix
	// As such we distinguish clearly between the metadata labels and the user defined labels in the spec
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// VLANDatabaseStatus defines the observed state of VLANDatabase
type VLANDatabaseStatus struct {
	// ConditionedStatus provides the status of the VLAN Database using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	allocv1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.kind=='Synced')].status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.conditions[?(@.kind=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={nephio,alloc}
// VLANDatabase is the Schema for the vlan database API
type VLANDatabase struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   VLANDatabaseSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status VLANDatabaseStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VLANDatabaseList contains a list of VLANDatabases
type VLANDatabaseList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []VLANDatabase `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&VLANDatabase{}, &VLANDatabaseList{})
}

var (
	VLANDatabaseKind             = reflect.TypeOf(VLANDatabase{}).Name()
	VLANDatabaseGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: VLANDatabaseKind}.String()
	VLANDatabaseKindAPIVersion   = VLANDatabaseKind + "." + GroupVersion.String()
	VLANDatabaseGroupVersionKind = GroupVersion.WithKind(VLANDatabaseKind)
	VLANDatabaseKindGVKString    = meta.GVKToString(&schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    VLANDatabaseKind,
	})
)
