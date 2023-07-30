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

package logicalinterconnect

import (
	"fmt"

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"k8s.io/utils/pointer"
)

type selectionContext struct {
	links uint16
	// selectedEndpoints keeps track of the selected endpoints
	// -> labeled after the selection completes
	selectedEndpoints []invv1alpha1.Endpoint
	// logicalEndpoints keeps track of the selected endpoints
	// logical links are per endpoint, the per node/topology
	// actuation happens by the logicalendpoint controller
	// -> applied after selection
	logicalEndpoints []invv1alpha1.LogicalEndpointSpec
	// links keeps track of the links
	// -> applied after selection
	linkSpecs []invv1alpha1.LinkSpec
}

func newSelectionContext(links uint16) *selectionContext {
	return &selectionContext{
		links:             links,
		selectedEndpoints: make([]invv1alpha1.Endpoint, 0, links*2),
		logicalEndpoints:  make([]invv1alpha1.LogicalEndpointSpec, 0, 2),
		linkSpecs:         make([]invv1alpha1.LinkSpec, links),
	}
}

func (r *selectionContext) validateIndices(linkIdx, epIdx int) error {
	if linkIdx < 0 || linkIdx >= int(r.links) {
		return fmt.Errorf("invalid linkIndex, got: %d, want linkidx > 0 and < %d", linkIdx, r.links)
	}
	if epIdx < 0 || epIdx >= 2 {
		return fmt.Errorf("invalid endpointIndex, got: %d, want endpointIndex > 0 and < 2", epIdx)
	}
	return nil
}

func (r *selectionContext) addEndpoint(topology string, linkIdx, epIdx int, ep invv1alpha1.Endpoint) error {
	if err := r.validateIndices(linkIdx, epIdx); err != nil {
		return err
	}
	// add selected endpoint
	r.selectedEndpoints = append(r.selectedEndpoints, ep)
	// add linkSpec
	if len(r.linkSpecs[linkIdx].Endpoints) == 0 {
		r.linkSpecs[linkIdx].Endpoints = make([]invv1alpha1.EndpointSpec, 2)
	}
	r.linkSpecs[linkIdx].Endpoints[epIdx] = invv1alpha1.EndpointSpec{
		Topology:      pointer.String(topology),
		InterfaceName: ep.Spec.InterfaceName,
		NodeName:      ep.Spec.NodeName,
	}
	// add logical endpoint
	if len(r.logicalEndpoints[epIdx].Endpoints) == 0 {
		r.logicalEndpoints[epIdx].Endpoints = []invv1alpha1.EndpointSpec{}
	}
	r.logicalEndpoints[epIdx].Endpoints = append(r.logicalEndpoints[epIdx].Endpoints,
		invv1alpha1.EndpointSpec{
			Topology:      pointer.String(topology),
			InterfaceName: ep.Spec.InterfaceName,
			NodeName:      ep.Spec.NodeName,
		},
	)
	return nil
}

func (r *selectionContext) getLinkSpecs() []invv1alpha1.LinkSpec {
	return r.linkSpecs
}

func (r *selectionContext) getSelectedEndpoints() []invv1alpha1.Endpoint {
	return r.selectedEndpoints
}

func (r *selectionContext) getLogicalEndpoints() []invv1alpha1.LogicalEndpointSpec {
	return r.logicalEndpoints
}
