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
	"fmt"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition  based on the conditionkind
func (r *VLANDatabase) GetCondition(ck allocv1alpha1.ConditionKind) allocv1alpha1.Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *VLANDatabase) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetNamespacedName returns the namespace and name of the resource
func (r *VLANDatabase) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *VLANDatabase) GetGenericNamespacedName() string {
	if r.GetNamespace() == "" {
		return r.GetName()
	}
	return fmt.Sprintf("%s-%s", r.GetNamespace(), r.GetName())
}

// GetLabels returns the user defined labels in the spec
func (r *VLANDatabase) GetSpecLabels() map[string]string {
	if len(r.Spec.Labels) == 0 {
		r.Spec.Labels = map[string]string{}
	}
	return r.Spec.Labels
}

func (r *VLANDatabase) GetCacheID() corev1.ObjectReference {
	return corev1.ObjectReference{Kind: string(r.Spec.VLANDBKind), Name: r.GetName(), Namespace: r.GetNamespace()}
}
