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
)

// GetCondition returns the condition based on the condition kind
func (r *VLAN) GetCondition(ck allocv1alpha1.ConditionKind) allocv1alpha1.Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *VLAN) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *VLAN) GetGenericNamespacedName() string {
	if r.GetNamespace() == "" {
		return r.GetName()
	}
	return fmt.Sprintf("%s-%s", r.GetNamespace(), r.GetName())
}

// GetVlanID return the vlanID from the spec
func (r *VLAN) GetVlanID() uint16 {
	return r.Spec.VlanID
}

// GetVlanID return the vlanID from the spec
func (r *VLAN) GetVlanRange() string {
	return r.Spec.VLANRange
}

// GetLabels returns the user defined labels in the spec
func (r *VLAN) GetSpecLabels() map[string]string {
	if len(r.Spec.Labels) == 0 {
		r.Spec.Labels = map[string]string{}
	}
	return r.Spec.Labels
}

// GetAllocatedVlanID return the allocated vlanID from the status
func (r *VLAN) GetAllocatedVlanID() uint16 {
	return r.Status.AllocatedVlanID
}

// GetAllocatedVlanRange return the allocated vlanRange from the status
func (r *VLAN) GetAllocatedVlanRange() string {
	return r.Status.AllocatedVlanRange
}

// SetAllocatedVlanID updates the AllocatedVlanID status
func (r *VLAN) SetAllocatedVlanID(id uint16) {
	r.Status.AllocatedVlanID = id
}

// SetAllocatedVlanID updates the AllocatedVlanID status
func (r *VLAN) SetAllocatedVlanRange(ra string) {
	r.Status.AllocatedVlanRange = ra
}
