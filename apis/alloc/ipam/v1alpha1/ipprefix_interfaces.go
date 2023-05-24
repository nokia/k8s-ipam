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

package v1alpha1

import (
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition based on the condition kind
func (r *IPPrefix) GetCondition(t allocv1alpha1.ConditionType) allocv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *IPPrefix) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *IPPrefix) GetGenericNamespacedName() string {
	return allocv1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetCacheID return the cache id validating the namespace
func (r *IPPrefix) GetCacheID() corev1.ObjectReference {
	return allocv1alpha1.GetCacheID(r.Spec.NetworkInstance)
}

// GetUserDefinedLabels returns the user defined labels in the spec
func (r *IPPrefix) GetUserDefinedLabels() map[string]string {
	return r.Spec.GetUserDefinedLabels()
}

// BuildIPPrefix returns an IP Prefix from a client Object a crName and
// an IpPrefix Spec/Status
func BuildIPPrefix(meta metav1.ObjectMeta, spec IPPrefixSpec, status IPPrefixStatus) *IPPrefix {
	return &IPPrefix{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       IPPrefixKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}
