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
	"strings"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition of a networkinstance based on the
// conditionkind
func (r *NetworkInstance) GetCondition(ck allocv1alpha1.ConditionKind) allocv1alpha1.Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions set the conditions of the networkinstance
func (r *NetworkInstance) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetNamespacedName returns the namespace and name of the networkinstance
func (r *NetworkInstance) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

// GetNameFromNetworkInstancePrefix return a network instance prefix name
// It provides a unique name by concatenating the networkinstance name
// with a predefined name (aggregate) and a prefix, compliant to the naming
// convention of k8s
func (r *NetworkInstance) GetNameFromNetworkInstancePrefix(prefix string) string {
	return fmt.Sprintf("%s-%s-%s", r.GetName(), "aggregate", strings.ReplaceAll(prefix, "/", "-"))
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *NetworkInstance) GetGenericNamespacedName() string {
	if r.GetNamespace() == "" {
		return r.GetName()
	}
	return fmt.Sprintf("%s-%s", r.GetNamespace(), r.GetName())
}

// GetPrefixes returns the prefixes defined in the networkinstance
func (r *NetworkInstance) GetPrefixes() []*Prefix {
	return r.Spec.Prefixes
}

// GetPrefix returns the prefix in cidr notation
func (r *Prefix) GetPrefix() string {
	return r.Prefix
}

// GetLabels returns the labels of the prefix in the networkinstance
func (r *Prefix) GetPrefixLabels() map[string]string {
	return r.Labels
}

// GetAllocatedPrefixes returns the allocated prefixes the ipam backend
// allocated to this IPPrefix
func (r *NetworkInstance) GetAllocatedPrefixes() []*Prefix {
	return r.Status.AllocatedPrefixes
}

func (r *NetworkInstance) GetCacheID() corev1.ObjectReference {
	return corev1.ObjectReference{Name: r.GetName(), Namespace: r.GetNamespace()}
}
