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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// GetCondition returns the condition based on the condition kind
func (r *VLANAllocation) GetCondition(ck allocv1alpha1.ConditionKind) allocv1alpha1.Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *VLANAllocation) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *VLANAllocation) GetGenericNamespacedName() string {
	if r.GetNamespace() == "" {
		return r.GetName()
	}
	return fmt.Sprintf("%s-%s", r.GetNamespace(), r.GetName())
}

// GetVlanID return the vlanID from the spec
func (r *VLANAllocation) GetVlanID() uint16 {
	return r.Spec.VlanID
}

// GetVlanID return the vlanID from the spec
func (r *VLANAllocation) GetVlanRange() string {
	return r.Spec.VLANRange
}

// GetLabels returns the user defined labels in the spec
func (r *VLANAllocation) GetSpecLabels() map[string]string {
	if len(r.Spec.Labels) == 0 {
		r.Spec.Labels = map[string]string{}
	}
	return r.Spec.Labels
}

// GetSelector returns the selector of the ip allocation as defined in the spec
// if undefined a nil pointer is returned
func (r *VLANAllocation) GetSelector() *metav1.LabelSelector {
	return r.Spec.Selector
}

// GetSelectorLabels returns the matchLabels of the selector as a map[atring]string
// if the selector is undefined an empty map is returned
func (r *VLANAllocation) GetSelectorLabels() map[string]string {
	l := map[string]string{}
	if r.Spec.Selector != nil {
		for k, v := range r.Spec.Selector.MatchLabels {
			l[k] = v
		}
	}
	return l
}

// GetLabelSelector returns a labels selector from the selector labels in the spec
func (r *VLANAllocation) GetLabelSelector() (labels.Selector, error) {
	l := r.GetSelectorLabels()
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

// GetOwnerSelector returns a label selector to find the owner in the ipam backend
func (r *VLANAllocation) GetOwnerSelector() (labels.Selector, error) {
	l := map[string]string{
		allocv1alpha1.NephioNsnNameKey:           r.Spec.Labels[allocv1alpha1.NephioNsnNameKey],
		allocv1alpha1.NephioNsnNamespaceKey:      r.Spec.Labels[allocv1alpha1.NephioNsnNamespaceKey],
		allocv1alpha1.NephioOwnerGvkKey:          r.Spec.Labels[allocv1alpha1.NephioOwnerGvkKey],
		allocv1alpha1.NephioOwnerNsnNameKey:      r.Spec.Labels[allocv1alpha1.NephioOwnerNsnNameKey],
		allocv1alpha1.NephioOwnerNsnNamespaceKey: r.Spec.Labels[allocv1alpha1.NephioOwnerNsnNamespaceKey],
	}

	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

// GetFullLabels returns a map with a combination of the user defined labels
// in the spec and the selector labels defined in the spec
func (r *VLANAllocation) GetFullLabels() map[string]string {
	l := make(map[string]string)
	for k, v := range r.GetSpecLabels() {
		l[k] = v
	}
	for k, v := range r.GetSelectorLabels() {
		l[k] = v
	}
	return l
}

// GetAllocatedVlanID return the allocated vlanID from the status
func (r *VLANAllocation) GetAllocatedVlanID() uint16 {
	return r.Status.AllocatedVlanID
}

// GetAllocatedVlanRange return the allocated vlanRange from the status
func (r *VLANAllocation) GetAllocatedVlanRange() string {
	return r.Status.AllocatedVlanRange
}

// SetAllocatedVlanID updates the AllocatedVlanID status
func (r *VLANAllocation) SetAllocatedVlanID(id uint16) {
	r.Status.AllocatedVlanID = id
}

// SetAllocatedVlanID updates the AllocatedVlanRange status
func (r *VLANAllocation) SetAllocatedVlanRange(ra string) {
	r.Status.AllocatedVlanRange = ra
}
