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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func GetGVKfromGVR(c *rest.Config, gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	httpClient, err := rest.HTTPClientFor(c)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(c, httpClient)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	gvk, err := mapper.KindFor(gvr)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return gvk, nil
}
