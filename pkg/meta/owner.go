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

package meta

import (
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

type OwnerRef struct {
	APIVersion string    `json:"apiVersion" yaml:"apiVersion"`
	Kind       string    `json:"kind" yaml:"kind"`
	Namespace  string    `json:"namespace" yaml:"namespace"`
	Name       string    `json:"name" yaml:"name"`
	UID        types.UID `json:"uid" yaml:"uid"`
}

func OwnerRefToString(ownerref OwnerRef) string {
	var sb strings.Builder
	if ownerref.APIVersion != "" {
		sb.WriteString("." + ownerref.APIVersion)
	}
	if ownerref.Kind != "" {
		sb.WriteString("." + ownerref.Kind)
	}
	if ownerref.Namespace != "" {
		sb.WriteString("." + ownerref.Namespace)
	}
	if ownerref.Name != "" {
		sb.WriteString("." + ownerref.Name)
	}
	/*
		if ownerref.UID != "" {
			sb.WriteString("." + string(ownerref.UID))
		}
	*/
	return sb.String()
}
