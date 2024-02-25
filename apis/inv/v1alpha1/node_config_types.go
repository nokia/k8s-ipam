/*
Copyright 2023 Nokia.

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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type NodeConfigSpec struct {
	// Provider specifies the provider implementing this node config.
	Provider string `json:"provider" yaml:"provider"`
	// Model encodes variants (e.g. srlinux ixr-d3, ixr-6e, etc)
	Model *string `json:"model,omitempty"`
	// Image used to bootup the container
	Image *string `json:"image,omitempty"`
	// StartupConfig is pointer to the config map thaat contains the startup config
	StartupConfig *string `json:"startupConfig,omitempty"`
	// license key from license secret that contains a license file
	LicenseKey *string `json:"licenseKey,omitempty"`
	// Resources defines the limits and requests as key/value
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Persistent Volumes
	PersistentVolumes []PersistentVolume `json:"persistentVolumes,omitempty"`
	// ParametersRef points to the vendor or implementation specific params for the
	// node.
	// +optional
	ParametersRef *corev1.ObjectReference `json:"parametersRef,omitempty" yaml:"parametersRef,omitempty"`
}

type PersistentVolume struct {
	Name      string              `json:"name"`
	MountPath string              `json:"mountPath"`
	Requests  corev1.ResourceList `json:"requests"`
}

//+kubebuilder:object:root=true

// NodeConfig is the Schema for the srlinux nodeconfig API.
type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeConfigSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// NodeConfigList contains a list of srlinux NodeConfigs.
type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeConfig{}, &NodeConfigList{})
}

// Node type metadata.
var (
	NodeConfigKind             = reflect.TypeOf(NodeConfig{}).Name()
	NodeConfigGroupKind        = schema.GroupKind{Group: GroupVersion.Group, Kind: NodeConfigKind}.String()
	NodeConfigKindAPIVersion   = NodeConfigKind + "." + GroupVersion.String()
	NodeConfigGroupVersionKind = GroupVersion.WithKind(NodeConfigKind)
)
