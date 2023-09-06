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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NodePoolSpec defines the desired state of NodePool
type NodePoolSpec struct {
	// Provider defines the provider implementing this nodepool.
	Provider string `json:"provider" yaml:"provider"`
	// UserDefinedLabels define metadata  associated to the resource.
	// defined in the spec to distingiush metadata labels from user defined labels
	resourcev1alpha1.UserDefinedLabels `json:",inline" yaml:",inline"`
	// Location defines the location information where this resource is located
	// in lon/lat coordinates
	Location *Location `json:"location,omitempty" yaml:"location,omitempty"`
	// NodeConfig provides a reference to a node config resource
	// only name is used, we expect the namespace to be the same as the nodepool
	NodeConfig *corev1.ObjectReference `json:"nodeConfig,omitempty" yaml:"nodeConfig,omitempty"`
}

// NodePoolStatus defines the observed state of NodePool
type NodePoolStatus struct {
	// ConditionedStatus provides the status of the NodePool using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	resourcev1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
	//
	UsedNodeModelRef  *corev1.ObjectReference `json:"usedNodeModelRef,omitempty" yaml:"usedNodeModelRef,omitempty"`
	UsedNodeConfigRef *corev1.ObjectReference `json:"usedNodeConfigRef,omitempty" yaml:"usedNodeConfigRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="TOPOLOGY",type="string",JSONPath=".metadata.namespace"
// +kubebuilder:resource:categories={nephio,inv}
// NodePool is the Schema for the NodePool API
type NodePool struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   NodePoolSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status NodePoolStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeList contains a list of NodePools
type NodePoolList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []NodePool `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&NodePool{}, &NodePoolList{})
}

var (
	NodePoolKind             = reflect.TypeOf(NodePool{}).Name()
	NodePoolGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: NodePoolKind}.String()
	NodePoolKindAPIVersion   = NodePoolKind + "." + GroupVersion.String()
	NodePoolGroupVersionKind = GroupVersion.WithKind(NodePoolKind)
	NodePoolKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    NodePoolKind,
	})
)
