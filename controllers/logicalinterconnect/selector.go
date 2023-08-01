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
	"context"
	"fmt"

	"github.com/go-logr/logr"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	topov1alpha1 "github.com/nokia/k8s-ipam/apis/topo/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/objects/endpoint"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type selector struct {
	endpoint endpoint.Endpoint
	links    uint16
	//leps     []invv1alpha1.LogicalEndpoint
	result *result

	l logr.Logger
}

type result struct {
	// selectedEndpoints keeps track of the selected endpoints
	// -> labeled after the selection completes
	endpoints []invv1alpha1.Endpoint
	// logicalEndpoints keeps track of the selected endpoints
	// logical links are per endpoint, the per node/topology
	// actuation happens by the logicalendpoint controller
	// -> applied after selection
	logicalEndpoints []invv1alpha1.LogicalEndpointSpec
	// links keeps track of the links
	// -> applied after selection
	linkSpecs []invv1alpha1.LinkSpec
}

func newSelector(links uint16, endpoint endpoint.Endpoint) *selector {
	return &selector{
		endpoint: endpoint,
		links:    links,
		result: &result{
			endpoints:        make([]invv1alpha1.Endpoint, 0, links*2),
			logicalEndpoints: make([]invv1alpha1.LogicalEndpointSpec, 2),
			linkSpecs:        make([]invv1alpha1.LinkSpec, links),
		},
	}
}

func (r *selector) validateIndices(linkIdx, epIdx int) error {
	if linkIdx < 0 || linkIdx >= int(r.links) {
		return fmt.Errorf("invalid linkIndex, got: %d, want linkidx > 0 and < %d", linkIdx, r.links)
	}
	if epIdx < 0 || epIdx >= 2 {
		return fmt.Errorf("invalid endpointIndex, got: %d, want endpointIndex > 0 and < 2", epIdx)
	}
	return nil
}

func (r *selector) addEndpoint(topology string, linkIdx, epIdx int, ep invv1alpha1.Endpoint, lacp *bool, name *string) error {
	if err := r.validateIndices(linkIdx, epIdx); err != nil {
		return err
	}
	// add selected endpoint
	r.result.endpoints = append(r.result.endpoints, ep)
	// add linkSpec
	if len(r.result.linkSpecs[linkIdx].Endpoints) == 0 {
		r.result.linkSpecs[linkIdx].Endpoints = make([]invv1alpha1.EndpointSpec, 2)
	}
	r.result.linkSpecs[linkIdx].Endpoints[epIdx] = invv1alpha1.EndpointSpec{
		Topology:      topology,
		InterfaceName: ep.Spec.InterfaceName,
		NodeName:      ep.Spec.NodeName,
	}
	// add logical endpoint
	r.result.logicalEndpoints[epIdx].Lacp = lacp
	r.result.logicalEndpoints[epIdx].Name = name
	if len(r.result.logicalEndpoints[epIdx].Endpoints) == 0 {
		r.result.logicalEndpoints[epIdx].Endpoints = []invv1alpha1.EndpointSpec{}
	}
	r.result.logicalEndpoints[epIdx].Endpoints = append(r.result.logicalEndpoints[epIdx].Endpoints,
		invv1alpha1.EndpointSpec{
			Topology:      topology,
			InterfaceName: ep.Spec.InterfaceName,
			NodeName:      ep.Spec.NodeName,
		},
	)
	return nil
}

func (r *selector) selectEndpoints(ctx context.Context, cr *topov1alpha1.LogicalInterconnect) (*result, error) {
	r.l = log.FromContext(ctx)
	// initialize the endpoint inventory based on the cr and topology information
	if err := r.endpoint.Init(ctx, cr.GetTopologies()); err != nil {
		return nil, err
	}

	// selection is run per link endpoint to ensure we take into account topology and node diversity
	for epIdx, ep := range cr.Spec.Endpoints {
		// for node diversity we keep track of the nodes per topology
		// that have been selected - key = topology
		selectedNodes := map[string][]string{}
		// retrieve endpoints per topology - key = topology
		topoEndpoints := map[string][]invv1alpha1.Endpoint{}
		for _, topology := range ep.Topologies {
			var err error
			topoEndpoints[topology], err = r.endpoint.GetTopologyEndpointsWithSelector(topology, ep.Selector)
			if err != nil {
				return nil, err
			}
			selectedNodes[topology] = []string{"", ""}
		}

		// allocate endpoint per link
		for linkIdx := 0; linkIdx < int(cr.Spec.Links); linkIdx++ {
			// topoIdx is used to find the topology we should use for the
			// endpoint selection
			// Current strategy we walk one by one through each topology
			topoIdx := linkIdx % len(ep.Topologies)
			topology := ep.Topologies[topoIdx]
			// selectedNodeIdx is used for node selection when node diversity is used
			// we assume max 2 nodes for node diversity
			selectedNodeIdx := 0
			// for a multi topology environment node selection is not applicable
			// (we ignore it)
			if len(ep.Topologies) == 1 {
				selectedNodeIdx = linkIdx % 2
			}

			// select an endpoint within a topology
			found := false
			for idx, tep := range topoEndpoints[topology] {
				//r.l.Info("selector topoEndpoints", "gvk", fmt.Sprintf("%s.%s.%s", tep.APIVersion, tep.Kind, tep.Name))
				if tep.WasAllocated(cr) {
					found = true
					if err := r.addEndpoint(topology, linkIdx, epIdx, tep, cr.Spec.Lacp, ep.Name); err != nil {
						return nil, err
					}
					// delete the endpoint from the list to avoid reselection
					topoEndpoints[topology] = append(topoEndpoints[topology][:idx], topoEndpoints[topology][idx+1:]...)
					break
				}

				// if the selectedNode was already selected, node diversity was already
				// checked so we need to allocate an endpoint on the
				selectedNode := selectedNodes[topology][selectedNodeIdx]
				if selectedNode != "" {
					// node selection was already done, we need to select an endpoint
					// on the same node that was not already allocated
					if tep.Spec.NodeName == selectedNode && !tep.IsAllocated(cr) {
						// the nodeName previously selected node matches
						// -> we need to continue searching for an endpoint
						// that matches the previous selected node
						found = true
						if err := r.addEndpoint(topology, linkIdx, epIdx, tep, cr.Spec.Lacp, ep.Name); err != nil {
							return nil, err
						}
						// delete the endpoint from the list to avoid reselection
						topoEndpoints[topology] = append(topoEndpoints[topology][:idx], topoEndpoints[topology][idx+1:]...)
						break
					}
				} else {
					// node selection is required
					if len(ep.Topologies) == 1 && ep.SelectorPolicy != nil && ep.SelectorPolicy.NodeDiversity != nil && *ep.SelectorPolicy.NodeDiversity > 1 {
						// we need to select a node - ensure the node we select is
						// node diverse from the previous selected nodes and not allocated
						if tep.IsNodeDiverse(selectedNodeIdx, selectedNodes[topology]) && !tep.IsAllocated(cr) {
							// the ep is node diverse and not allocated -> we allocated this endpoint
							found = true
							selectedNodes[topology][selectedNodeIdx] = tep.Spec.NodeName
							if err := r.addEndpoint(topology, linkIdx, epIdx, tep, cr.Spec.Lacp, ep.Name); err != nil {
								return nil, err
							}
							// delete the endpoint from the list to avoid reselection
							topoEndpoints[topology] = append(topoEndpoints[topology][:idx], topoEndpoints[topology][idx+1:]...)
							break
						}
					} else {
						if !tep.IsAllocated(cr) {
							found = true
							selectedNodes[topology][selectedNodeIdx] = tep.Spec.NodeName
							if err := r.addEndpoint(topology, linkIdx, epIdx, tep, cr.Spec.Lacp, ep.Name); err != nil {
								return nil, err
							}
							// delete the endpoint from the list to avoid reselection
							topoEndpoints[topology] = append(topoEndpoints[topology][:idx], topoEndpoints[topology][idx+1:]...)
							break
						}
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("no endpoints available")
			}
		}
	}

	return r.result, nil
}

func (r *result) getLinkSpecs() []invv1alpha1.LinkSpec {
	return r.linkSpecs
}

func (r *result) getSelectedEndpoints() []invv1alpha1.Endpoint {
	return r.endpoints
}

func (r *result) getLogicalEndpoints() []invv1alpha1.LogicalEndpointSpec {
	return r.logicalEndpoints
}

// option 1:
// give the endpoints back -> issue is reallocation

// option 2:
// list the allocated endpoints based on links or endpoints
// check if the selection
