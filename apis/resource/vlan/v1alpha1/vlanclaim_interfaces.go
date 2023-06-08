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

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition based on the condition kind
func (r *VLANClaim) GetCondition(t resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *VLANClaim) SetConditions(c ...resourcev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *VLANClaim) GetGenericNamespacedName() string {
	return resourcev1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetCacheID return the cache id validating the namespace
func (r *VLANClaim) GetCacheID() corev1.ObjectReference {
	return resourcev1alpha1.GetCacheID(r.Spec.VLANIndex)
}

// GetUserDefinedLabels returns a map with a copy of the user defined labels
func (r *VLANClaim) GetUserDefinedLabels() map[string]string {
	return r.Spec.GetUserDefinedLabels()
}

// GetSelectorLabels returns a map with a copy of the selector labels
func (r *VLANClaim) GetSelectorLabels() map[string]string {
	return r.Spec.GetSelectorLabels()
}

// GetFullLabels returns a map with a copy of the user defined labels and the selector labels
func (r *VLANClaim) GetFullLabels() map[string]string {
	return r.Spec.GetFullLabels()
}

// GetLabelSelector returns a labels selector based on the label selector
func (r *VLANClaim) GetLabelSelector() (labels.Selector, error) {
	return r.Spec.GetLabelSelector()
}

// GetOwnerSelector returns a label selector to select the owner of the claim in the backend
func (r *VLANClaim) GetOwnerSelector() (labels.Selector, error) {
	return r.Spec.GetOwnerSelector()
}

type VLANClaimCtx struct {
	Kind  VLANClaimType
	Start uint16
	Size  uint16
}

func (r *VLANClaim) GetVLANClaimCtx() (*VLANClaimCtx, error) {
	vlanClaimCtx := &VLANClaimCtx{
		Kind: VLANClaimTypeDynamic,
	}
	switch {
	case r.Spec.VLANID != nil:
		vlanClaimCtx.Kind = VLANClaimTypeStatic
		vlanClaimCtx.Start = *r.Spec.VLANID
	case r.Spec.VLANRange != nil:
		// range can be defined in 2 ways
		// size -> 100, in this case we dont care if they are claimed consecutively or not
		// start:stop -> 100:200, here the full range should be claimed consecutively
		split := strings.Split(*r.Spec.VLANRange, ":")
		switch {
		case len(split) == 1: // size based range
			vlanClaimCtx.Kind = VLANClaimTypeSize
			s, err := strconv.Atoi(split[0])
			if err != nil {
				return nil, err
			}
			vlanClaimCtx.Size = uint16(s)
		case len(split) == 2: // start:stop range
			vlanClaimCtx.Kind = VLANClaimTypeRange
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
			vlanClaimCtx.Start = uint16(s)
			vlanClaimCtx.Size = uint16(e) - uint16(s) + 1
		default:
			return nil, fmt.Errorf("VLAN range can either be start:end or size, got: %s", *r.Spec.VLANRange)
		}
	}
	return vlanClaimCtx, nil
}

// AddOwnerLabelsToCR returns a VLAN Claim
// by augmenting the owner GVK/NSN in the user defined labels
func (r *VLANClaim) AddOwnerLabelsToCR() {
	if r.Spec.UserDefinedLabels.Labels == nil {
		r.Spec.UserDefinedLabels.Labels = map[string]string{}
	}
	for k, v := range resourcev1alpha1.GetHierOwnerLabelsFromCR(r) {
		r.Spec.UserDefinedLabels.Labels[k] = v
	}
}

// BuildVLANClaim returns a VLANClaim from a client Object a crName and
// an VLANClaim Spec/Status
func BuildVLANClaim(meta metav1.ObjectMeta, spec VLANClaimSpec, status VLANClaimStatus) *VLANClaim {
	return &VLANClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       VLANClaimKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}
