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

// GetCondition returns the condition based on the condition kind
//func (r *Endpoint) GetCondition(t resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
//	return r.Status.GetCondition(t)
//}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
//func (r *Endpoint) SetConditions(c ...resourcev1alpha1.Condition) {
//	r.Status.SetConditions(c...)
//}

func (r *EndpointList) GetItems() []client.Object {
	objs := []client.Object{}
	for _, ep := range r.Items {
		x := ep
		objs = append(objs, &x)
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
	return r.Labels[NephioInventoryLinkNameKey]
}

// IsNodeDiverse returns false if a the endpoint nodeName matches
// the name in the node slice
// Only the
func (r *Endpoint) IsNodeDiverse(idx int, nodes []string) bool {
	for nodeIdx, nodeName := range nodes {
		// only for the nodes
		if nodeIdx != idx && nodeName != "" && nodeName == r.Spec.NodeName {
			return false
		}
	}
	return true
}

// IsAllocated returns a bool that indicates if the endpoint
// was already allocated by another owner
func (r *Endpoint) IsAllocated(cr client.Object) bool {
	if r.Status.ClaimRef == nil {
		return false
	}
	if r.Status.ClaimRef.APIVersion == cr.GetObjectKind().GroupVersionKind().GroupVersion().String() &&
		r.Status.ClaimRef.APIVersion == cr.GetObjectKind().GroupVersionKind().Kind &&
		r.Status.ClaimRef.Namespace == cr.GetNamespace() &&
		r.Status.ClaimRef.Name == cr.GetName() {
		return false
	}
	return true
}

// WasAllocated returns a bool that indicates if the endpoint
// was already allocated by this CR
func (r *Endpoint) WasAllocated(cr client.Object) bool {
	if r.Status.ClaimRef == nil {
		return false
	}
	if r.Status.ClaimRef.APIVersion == cr.GetObjectKind().GroupVersionKind().GroupVersion().String() &&
		r.Status.ClaimRef.APIVersion == cr.GetObjectKind().GroupVersionKind().Kind &&
		r.Status.ClaimRef.Namespace == cr.GetNamespace() &&
		r.Status.ClaimRef.Name == cr.GetName() {
		return true
	}
	return false
}
