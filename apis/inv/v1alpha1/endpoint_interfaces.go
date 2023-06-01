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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *EndpointList) GetItems() []client.Object {
	objs := []client.Object{}
	for _, r := range r.Items {
		objs = append(objs, &r)
	}
	return objs
}

// BuildEndpoint returns a Endpoint from a client Object a crName and
// an Endpoint Spec/Status
func BuildEndpoint(meta metav1.ObjectMeta, spec EndpointSpec, status EndpointStatus) *Endpoint {
	return &Endpoint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       EndpointKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}

// GetLinkName returns a string representing the link the endpoint belongs to
// combined with the index of the link
func (r *Endpoint) GetLinkName() string {
	return r.Labels[NephioLinkNameKey]
}
