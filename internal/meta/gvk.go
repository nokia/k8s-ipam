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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GVKToString(gvk *schema.GroupVersionKind) string {
	return fmt.Sprintf("%s.%s.%s", gvk.Kind, gvk.Version, gvk.Group)
}

func StringToGVK(s string) *schema.GroupVersionKind {
	var gvk *schema.GroupVersionKind
	if strings.Count(s, ".") >= 2 {
		s := strings.SplitN(s, ".", 3)
		gvk = &schema.GroupVersionKind{Group: s[2], Version: s[1], Kind: s[0]}
	}
	return gvk
}

func apiVersionToGroupVersion(apiVersion string) (string, string) {
	split := strings.Split(apiVersion, "/")
	if len(split) > 1 {
		return split[0], split[1]
	}
	return "", apiVersion
}

func GetGVKFromAPIVersionKind(apiVersion, kind string) *schema.GroupVersionKind {
	ownerGroup, ownerVersion := apiVersionToGroupVersion(apiVersion)

	return &schema.GroupVersionKind{
		Group: ownerGroup, Version: ownerVersion, Kind: kind}
}

func GetGVKFromObject(o client.Object) *schema.GroupVersionKind {
	return &schema.GroupVersionKind{
		Group:   o.GetObjectKind().GroupVersionKind().Group,
		Version: o.GetObjectKind().GroupVersionKind().Version,
		Kind:    o.GetObjectKind().GroupVersionKind().Kind,
	}
}
