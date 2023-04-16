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
)

// GetCondition of this resource
func (r *IPPrefix) GetCondition(ck allocv1alpha1.ConditionKind) allocv1alpha1.Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions of the Network Node.
func (r *IPPrefix) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *IPPrefix) GetGenericNamespacedName() string {
	if r.GetNamespace() == "" {
		return r.GetName()
	}
	return fmt.Sprintf("%s-%s", r.GetNamespace(), r.GetName())
}

// GetPrefixKind returns the prefixkind of the ip prefix
func (r *IPPrefix) GetPrefixKind() PrefixKind {
	return r.Spec.PrefixKind
}

// GetNetworkInstance returns the networkinstance of the ipprefix
func (r *IPPrefix) GetNetworkInstance() corev1.ObjectReference {
	nsn := corev1.ObjectReference{}
	if r.Spec.NetworkInstance != nil {
		nsn.Name = r.Spec.NetworkInstance.Name
		nsn.Namespace = r.Spec.NetworkInstance.Namespace
	}
	return nsn
}

// GetPrefix returns the prefix of the IPPrefix in cidr notation
// if the prefix was undefined and empty string is returned
func (r *IPPrefix) GetPrefix() string {
	return r.Spec.Prefix
}

// GetLabels returns the user defined labels of the IPPrefix
// defined in the spec
func (r *IPPrefix) GetSpecLabels() map[string]string {
	if len(r.Spec.Labels) == 0 {
		r.Spec.Labels = map[string]string{}
	}
	return r.Spec.Labels
}

// GetAllocatedPrefixes returns the allocated prefixes the ipam backend
// allocated to this IPPrefix
func (r *IPPrefix) GetAllocatedPrefix() string {
	return r.Status.AllocatedPrefix
}
