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
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition  based on the condition type
func (r *VXLANIndex) GetCondition(ct resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
	return r.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *VXLANIndex) SetConditions(c ...resourcev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetNamespacedName returns the namespace and name
func (r *VXLANIndex) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *VXLANIndex) GetGenericNamespacedName() string {
	return resourcev1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetUserDefinedLabels returns the user defined labels in the spec
func (r *VXLANIndex) GetUserDefinedLabels() map[string]string {
	return r.Spec.GetUserDefinedLabels()
}

// GetCacheID returns a CacheID as an objectReference
func (r *VXLANIndex) GetCacheID() corev1.ObjectReference {
	return resourcev1alpha1.GetCacheID(corev1.ObjectReference{Name: r.GetName(), Namespace: r.GetNamespace()})
}

// BuildVXLANIndex returns a VXLANIndex from a client Object a crName and
// an VXLANIndex Spec/Status
func BuildVXLANIndex(meta metav1.ObjectMeta, spec VXLANIndexSpec, status VXLANIndexStatus) *VXLANIndex {
	return &VXLANIndex{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       VXLANIndexKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}
