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

package ipam

import (
	"context"
	"fmt"
	"strings"

	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/pkg/errors"
	"inet.af/netaddr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type InsertorFn func(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error)

func (r *ipam) applyAllocation(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error) {
	r.im.Lock()
	insertFn := r.insertor[ipamUsage{PrefixKind: alloc.PrefixKind, HasPrefix: alloc.Prefix != ""}]
	r.im.Unlock()
	return insertFn(ctx, alloc)
}

func (r *ipam) NopPrefixInsertor(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error) {
	return &AllocatedPrefix{}, nil
}

func (r *ipam) GenericPrefixInsertor(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("insert prefix", "kind", alloc.PrefixKind, "alloc", alloc)

	rt, err := r.getRoutingTable(alloc, false)
	if err != nil {
		return nil, err
	}

	// get route
	p := alloc.GetIPPrefix()
	_, ok, err := rt.Get(p)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get ip prefix")
	}

	//if route exists -> update
	// if route does not exist -> add
	route := table.NewRoute(p)
	route.UpdateLabel(alloc.GetFullLabels())
	if ok {
		// prefix exists -> update
		if err := rt.Update(route); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "cannot update prefix")
			}
		}
	} else {
		// prefix does not exists -> add
		if err := rt.Add(route); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "cannot add prefix")
			}
		}
	}

	return &AllocatedPrefix{
		AllocatedPrefix: alloc.GetPrefixFromNewAlloc(),
	}, nil
}

func (r *ipam) NopPrefixAllocator(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error) {
	return &AllocatedPrefix{}, nil
}

func (r *ipam) GenericPrefixAllocator(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("allocate prefix", "kind", alloc.PrefixKind, "alloc", alloc)

	rt, err := r.getRoutingTable(alloc, false)
	if err != nil {
		return nil, err
	}

	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	allocSelector, err := alloc.GetAllocSelector()
	if err != nil {
		return nil, err
	}
	routes := rt.GetByLabel(allocSelector)
	if len(routes) != 0 {
		// there should only be 1 route with this name in the route table
		route := routes[0]
		route.UpdateLabel(alloc.GetFullLabels())
		// update the route with the latest labels
		if err := rt.Update(route); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "route insertion failed")
			}
		}
		p := route.IPPrefix()
		allocatedPrefix := &AllocatedPrefix{
			AllocatedPrefix: p.String(),
		}

		if alloc.PrefixKind == ipamv1alpha1.PrefixKindNetwork {
			gatewaySelector, err := alloc.GetGatewayLabelSelector()
			if err != nil {
				return nil, err
			}
			r.l.Info("gateway", "gatewaySelector", gatewaySelector)
			routes = rt.GetByLabel(gatewaySelector)
			if len(routes) > 0 {
				r.l.Info("gateway", "routes", routes)
				allocatedPrefix.Gateway = strings.Split(routes[0].IPPrefix().String(), "/")[0]
			}
		}
		r.l.Info("allocatedPrefix", "allocatedPrefix", allocatedPrefix)
		return allocatedPrefix, nil

	}
	// A NEW Allocation is required
	labelSelector, err := alloc.GetLabelSelector()
	if err != nil {
		return nil, err
	}

	routes = rt.GetByLabel(labelSelector)
	if len(routes) == 0 {
		return nil, fmt.Errorf("no available routes based on the label selector: %v", labelSelector)
	}

	prefixLength := alloc.GetPrefixLengthFromRoute(routes[0])
	selectedRoute := r.GetSelectedRouteWithPrefixLength(routes, prefixLength)
	if selectedRoute == nil {
		return nil, fmt.Errorf("no route found with requested prefixLength: %d", prefixLength)
	}
	p, ok := rt.FindFreePrefix(selectedRoute.IPPrefix(), prefixLength)
	if !ok {
		return nil, errors.New("no free prefix found")
	}
	route := table.NewRoute(p)
	prefix := p.String()
	labels := alloc.GetFullLabels()
	// since this is a dynamic allocation we dont know the prefix info ahead of time,
	// we need to augment the labels on the routes with thi info
	if prefixLength == 32 || prefixLength == 128 {
		labels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.GetPrefixLength(selectedRoute.IPPrefix())
		n := strings.Split(prefix, "/")
		prefix = strings.Join([]string{n[0], iputil.GetPrefixLength(selectedRoute.IPPrefix())}, "/")
	}
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(selectedRoute.IPPrefix()))
	labels[ipamv1alpha1.NephioPrefixLengthKey] = string(iputil.GetAddressPrefixLength(p))
	labels[ipamv1alpha1.NephioNetworkKey] = selectedRoute.IPPrefix().Masked().IP().String()
	route.UpdateLabel(labels)

	if err := rt.Update(route); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return nil, errors.Wrap(err, "cannot add prefix")
		}
	}

	allocatedPrefix := &AllocatedPrefix{
		AllocatedPrefix: prefix,
	}

	if alloc.PrefixKind == ipamv1alpha1.PrefixKindNetwork {
		gatewaySelector, err := alloc.GetGatewayLabelSelector()
		if err != nil {
			return nil, err
		}
		r.l.Info("gateway", "gatewaySelector", gatewaySelector)
		routes = rt.GetByLabel(gatewaySelector)
		if len(routes) > 0 {
			r.l.Info("gateway", "routes", routes)
			allocatedPrefix.Gateway = strings.Split(routes[0].IPPrefix().String(), "/")[0]
		}
	}

	r.l.Info("allocatedPrefix", "allocatedPrefix", allocatedPrefix)
	return allocatedPrefix, nil
}

func (r *ipam) GetSelectedRouteWithPrefixLength(routes table.Routes, prefixLength uint8) *table.Route {
	r.l.Info("alloc w/o prefix", "routes", routes)

	var selectedRoute *table.Route
	if prefixLength == 32 || prefixLength == 128 {
		selectedRoute = routes[0]
	} else {
		for _, route := range routes {
			ps, _ := netaddr.MustParseIPPrefix(route.String()).IPNet().Mask.Size()
			if ps < int(prefixLength) {
				selectedRoute = route
				break
			}
		}
	}
	return selectedRoute
}
