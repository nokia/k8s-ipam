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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

func GetUnstructuredFromGVK(gvk *schema.GroupVersionKind) *unstructured.Unstructured {
	var u unstructured.Unstructured
	u.SetAPIVersion(gvk.GroupVersion().String())
	u.SetKind(gvk.Kind)
	uCopy := u.DeepCopy()
	return uCopy
}

func GetUnstructuredListFromGVK(gvk *schema.GroupVersionKind) *unstructured.UnstructuredList {
	var u unstructured.UnstructuredList
	u.SetAPIVersion(gvk.GroupVersion().String())
	u.SetKind(gvk.Kind)
	uCopy := u.DeepCopy()
	return uCopy
}

func MarshalData(o *unstructured.Unstructured) (any, error) {
	b, err := yaml.Marshal(o.UnstructuredContent())
	if err != nil {
		return nil, err
	}

	rj := map[string]interface{}{}
	if err := yaml.Unmarshal(b, &rj); err != nil {
		return nil, err
	}
	return rj, nil
}
