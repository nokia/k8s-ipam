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
	"fmt"
	"strconv"
	"strings"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition based on the condition kind
func (r *VLANAllocation) GetCondition(t allocv1alpha1.ConditionType) allocv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *VLANAllocation) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *VLANAllocation) GetGenericNamespacedName() string {
	return allocv1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetUserDefinedLabels returns a map with a copy of the user defined labels
func (r *VLANAllocation) GetUserDefinedLabels() map[string]string {
	return r.Spec.GetUserDefinedLabels()
}

// GetSelectorLabels returns a map with a copy of the selector labels
func (r *VLANAllocation) GetSelectorLabels() map[string]string {
	return r.Spec.GetSelectorLabels()
}

// GetFullLabels returns a map with a copy of the user defined labels and the selector labels
func (r *VLANAllocation) GetFullLabels() map[string]string {
	return r.Spec.GetFullLabels()
}

// GetLabelSelector returns a labels selector based on the label selector
func (r *VLANAllocation) GetLabelSelector() (labels.Selector, error) {
	return r.Spec.GetLabelSelector()
}

// GetOwnerSelector returns a label selector to select the owner of the allocation in the backend
func (r *VLANAllocation) GetOwnerSelector() (labels.Selector, error) {
	return r.Spec.GetOwnerSelector()
}

type VLANAllocationCtx struct {
	Kind  VLANAllocKind
	Start uint16
	Size  uint16
}

func (r *VLANAllocation) GetVLANAllocationCtx() (*VLANAllocationCtx, error) {
	vlanAllocCtx := &VLANAllocationCtx{
		Kind: VLANAllocKindDynamic,
	}
	switch {
	case r.Spec.VLANID != nil:
		vlanAllocCtx.Kind = VLANAllocKindStatic
		vlanAllocCtx.Start = *r.Spec.VLANID
	case r.Spec.VLANRange != nil:
		// range can be defined in 2 ways
		// size -> 100, in this case we dont care if they are allocated consecutively or not
		// start:stop -> 100:200, here the full range should be allocated consecutively
		split := strings.Split(*r.Spec.VLANRange, ":")
		switch {
		case len(split) == 1: // size based range
			vlanAllocCtx.Kind = VLANAllocKindSize
			s, err := strconv.Atoi(split[0])
			if err != nil {
				return nil, err
			}
			vlanAllocCtx.Size = uint16(s)
		case len(split) == 2: // start:stop range
			vlanAllocCtx.Kind = VLANAllocKindRange
			s, err := strconv.Atoi(split[0])
			if err != nil {
				return nil, err
			}
			e, err := strconv.Atoi(split[1])
			if err != nil {
				return nil, err
			}
			if e < s {
				return nil, fmt.Errorf("VLAN range %s end %d can not be smaller than start %d", *r.Spec.VLANRange, e, s)
			}
			vlanAllocCtx.Start = uint16(s)
			vlanAllocCtx.Size = uint16(e) - uint16(s) + 1
		default:
			return nil, fmt.Errorf("VLAN range can either be start:end or size, got: %s", *r.Spec.VLANRange)
		}
	}
	return vlanAllocCtx, nil
}

// AddOwnerLabelsToCR returns a VLANAllocation
// by augmenting the owner GVK/NSN in the user defined labels
func (r *VLANAllocation) AddOwnerLabelsToCR() {
	if r.Spec.UserDefinedLabels.Labels == nil {
		r.Spec.UserDefinedLabels.Labels = map[string]string{}
	}
	for k, v := range allocv1alpha1.GetOwnerLabelsFromCR(r) {
		r.Spec.UserDefinedLabels.Labels[k] = v
	}
}
