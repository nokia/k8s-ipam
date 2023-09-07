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

func (r *OwnerRef) String() string {
	var sb strings.Builder
	sb.WriteString("ref")
	if r.APIVersion != "" {
		sb.WriteString("." + r.APIVersion)
	}
	if r.Kind != "" {
		sb.WriteString("." + r.Kind)
	}
	if r.Namespace != "" {
		sb.WriteString("." + r.Namespace)
	}
	if r.Name != "" {
		sb.WriteString("." + r.Name)
	}
	/*
		if ownerref.UID != "" {
			sb.WriteString("." + string(ownerref.UID))
		}
	*/
	return strings.ReplaceAll(sb.String(), "/", "_")
}
