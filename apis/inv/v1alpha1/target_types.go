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

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Encoding string

const (
	Encoding_Unknown   Encoding = "unknown"
	Encoding_JSON      Encoding = "JSON"
	Encoding_JSON_IETF Encoding = "JSON_IETF"
	Encoding_Bytes     Encoding = "bytes"
	Encoding_Protobuf  Encoding = "protobuf"
	Encoding_Ascii     Encoding = "ASCII"
)

type Protocol string

const (
	Protocol_Unknown Encoding = "unknown"
	Protocol_GNMI    Encoding = "gnmi"
	Protocol_NETCONF Encoding = "netconf"
)

// TargetSpec defines the desired state of Target
type TargetSpec struct {
	// Provider specifies the provider using this target.
	Provider string `json:"provider" yaml:"provider"`
	// ParametersRef points to the vendor or implementation specific params for the
	// target.
	// +optional
	ParametersRef *corev1.ObjectReference `json:"parametersRef,omitempty" yaml:"parametersRef,omitempty"`

	Address    *string `json:"address,omitempty" yaml:"address,omitempty"`
	SecretName string  `json:"secretName" yaml:"secretName"`
	//+kubebuilder:validation:Enum=unknown;JSON;JSON_IETF;bytes;protobuf;ASCII;
	Encoding *Encoding `json:"encoding,omitempty" yaml:"encoding,omitempty"`
	Insecure *bool     `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	//+kubebuilder:validation:Enum=unknown;gnmi;netconf;
	Protocol      *Protocol `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	SkipVerify    *bool     `json:"skipVerify,omitempty" yaml:"skipVerify,omitempty"`
	TLSSecretName *string   `json:"tlsSecretName,omitempty" yaml:"tlsSecretName,omitempty"`
}

// TargetStatus defines the observed state of Target
type TargetStatus struct {
	// ConditionedStatus provides the status of the Target allocation using conditions
	// 2 conditions are used:
	// - a condition for the reconcilation status
	// - a condition for the ready status
	// if both are true the other attributes in the status are meaningful
	allocv1alpha1.ConditionedStatus `json:",inline" yaml:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={nephio,inv}
// Target is the Schema for the vlan API
type Target struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   TargetSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status TargetStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TargetList contains a list of Targets
type TargetList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []Target `json:"items" yaml:"items"`
}

func init() {
	SchemeBuilder.Register(&Target{}, &TargetList{})
}

var (
	TargetKind             = reflect.TypeOf(Target{}).Name()
	TargetGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: TargetKind}.String()
	TargetKindAPIVersion   = TargetKind + "." + GroupVersion.String()
	TargetGroupVersionKind = GroupVersion.WithKind(TargetKind)
	TargetKindGVKString    = meta.GVKToString(schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    TargetKind,
	})
)
