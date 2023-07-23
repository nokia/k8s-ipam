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

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetCondition returns the condition based on the condition kind
func (r *Interconnect) GetCondition(t resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *Interconnect) SetConditions(c ...resourcev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

func (r *Interconnect) GetTopology(linkIdx, epIdx int) (string, error) {
	if err := r.validateIndices(linkIdx, epIdx); err != nil {
		return "", err
	}
	if r.Spec.Links[linkIdx].Endpoints[epIdx].Topology != nil {
		// return a specific topology link
		return *r.Spec.Links[linkIdx].Endpoints[epIdx].Topology, nil
	}
	// global topology link
	if epIdx >= len(r.Spec.Topologies) {
		return "", fmt.Errorf("gettopology requested a global topology but the global topology data is not providing this index, got epIndex: %d, topology data len: %d", epIdx, len(r.Spec.Topologies))
	}
	return r.Spec.Topologies[epIdx], nil
}

func (r *Interconnect) GetEndpointSelector(linkIdx int, epIdx int) (*metav1.LabelSelector, error) {
	if err := r.validateIndices(linkIdx, epIdx); err != nil {
		return nil, err
	}
	ep := r.Spec.Links[linkIdx].Endpoints[epIdx]

	selector := &metav1.LabelSelector{}
	if ep.Selector != nil {
		selector = ep.Selector
	} else {
		if ep.InterfaceName == nil || ep.NodeName == nil {
			return nil, fmt.Errorf("specific endpoint linkindex: %d, epIndex: %d selector requested but interfaceName or nodeName nil", linkIdx, epIdx)
		}
		selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				invv1alpha1.NephioInventoryInterfaceNameKey: *ep.InterfaceName,
				invv1alpha1.NephioInventoryNodeNameKey:      *ep.NodeName,
			},
		}
	}
	return selector, nil

}

func (r *Interconnect) validateIndices(linkIdx, epIdx int) error {
	if err := r.validateLinkIndex(linkIdx); err != nil {
		return err
	}
	if epIdx >= len(r.Spec.Links[linkIdx].Endpoints) {
		return fmt.Errorf("gettopology requested with wrong epIndex got: %d, len: %d on linkindex: %d", linkIdx, len(r.Spec.Links), linkIdx)
	}
	return nil
}

func (r *Interconnect) validateLinkIndex(linkIdx int) error {
	if linkIdx >= len(r.Spec.Links) {
		return fmt.Errorf("gettopology requested with wrong linkIndex got: %d, len: %d", linkIdx, len(r.Spec.Links))
	}
	return nil
}

func (r *Interconnect) IsAllocated(epLabels map[string]string) bool {
	for k, v := range epLabels {
		if k == resourcev1alpha1.NephioOwnerGvkKey && v != meta.GVKToString(InterconnectGroupVersionKind) {
			// allocated by another resource
			return true
		}
		if k == resourcev1alpha1.NephioOwnerNsnNameKey && v != r.GetName() {
			// allocated by another resource
			return true
		}
		if k == resourcev1alpha1.NephioOwnerNsnNameKey && v != r.GetName() {
			// allocated by another resource
			return true
		}
	}
	return false
}

func (r *Interconnect) GetLogicalLinkIndex(linkIdx int) int {
	if err := r.validateLinkIndex(linkIdx); err != nil {
		return -1
	}
	if r.Spec.Links[linkIdx].Links != nil {
		return linkIdx
	}
	if r.Spec.Links[linkIdx].LogicalLinkIndex != nil {
		return *r.Spec.Links[linkIdx].LogicalLinkIndex
	}
	return -1
}
