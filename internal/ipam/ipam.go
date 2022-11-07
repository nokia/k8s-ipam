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
	"sync"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/allocator"
	"github.com/henderiw-nephio/ipam/internal/utils/iputil"
	"github.com/pkg/errors"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Option can be used to manipulate Options.
type Option func(Ipam)

type Ipam interface {
	Get(string) (*table.RouteTable, bool)
	// Create and initialize an ipam instance
	Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
	// Delete the ipam instance
	Delete(string)
	// AllocateIPPrefix allocates an ip prefix
	AllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation, dryrun bool, rt *table.RouteTable) (*AllocatedPrefix, error)
	// DeAllocateIPPrefixes deallocates ip prefixes
	DeAllocateIPPrefixes(ctx context.Context, allocs ...*ipamv1alpha1.IPAllocation) error
	// IsLatestPrefixInNetwork validates if this is the latest prefix in the network
	IsLatestPrefixInNetwork(cr *ipamv1alpha1.IPPrefix) bool
}

func New(c client.Client, a allocator.Allocator, opts ...Option) Ipam {
	i := &ipam{
		c:         c,
		allocator: a,
		ipam:      make(map[string]*table.RouteTable),
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

type ipam struct {
	c    client.Client
	m    sync.Mutex
	ipam map[string]*table.RouteTable

	allocator allocator.Allocator

	l logr.Logger
}

func (r *ipam) Get(crName string) (*table.RouteTable, bool) {
	r.m.Lock()
	defer r.m.Unlock()
	rt, ok := r.ipam[crName]
	return rt, ok
}

// Create an ipam instance and initializes it from the old state
func (r *ipam) Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {

	r.l = log.FromContext(context.Background())

	crName := types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}

	// initialize the ipam if the ipam instance does not exist
	if _, ok := r.ipam[crName.String()]; !ok {

		r.l = log.FromContext(context.Background())
		r.l.Info("ipam action", "action", "initialize", "name", crName, "ipam", r.ipam)

		r.m.Lock()
		r.ipam[crName.String()] = table.NewRouteTable()
		r.m.Unlock()

		// TODO review how to restore existing state -> current APPROACH is not bullet proof

		// list all prefixes
		prefixList := &ipamv1alpha1.IPPrefixList{}
		if err := r.c.List(context.Background(), prefixList); err != nil {
			return errors.Wrap(err, "cannot get ip prefix list")
		}

		// list all allocation to restore them in the ipam upon restart
		allocList := &ipamv1alpha1.IPAllocationList{}
		if err := r.c.List(context.Background(), allocList); err != nil {
			return errors.Wrap(err, "cannot get ip allocation list")
		}

		for prefix, labels := range cr.Status.Allocations {
			switch labels.Get(ipamv1alpha1.NephioOriginKey) {
			case string(ipamv1alpha1.OriginIPPrefix):
				for _, p := range prefixList.Items {
					if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == p.Name {
						if prefix != p.Spec.Prefix {
							r.l.Info("ipam action", "action", "initialize", "prefix", "match error", "ni prefix", prefix, "ipprefix prefix", p.Spec.Prefix, "kind", p.Spec.PrefixKind)
						}

						allocs := r.allocator.Allocate(&p)
						for _, alloc := range allocs {
							_, err := r.AllocateIPPrefix(ctx, alloc, false, nil)
							if err != nil {
								return err
							}
							/*
								a := netaddr.MustParseIPPrefix(alloc.Spec.Prefix)
								route := table.NewRoute(a)
								route.UpdateLabel(labels)
								//r.m.Lock()
								r.ipam[crName.String()].Add(route)
								//r.m.Unlock()
							*/
							r.l.Info("ipam action", "action", "initialize", "added prefix", alloc.Spec.Prefix)
						}
					}
				}
			case string(ipamv1alpha1.OriginIPAllocation):
				for _, alloc := range allocList.Items {
					if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == alloc.Name {
						if prefix != alloc.Spec.Prefix {
							r.l.Info("ipam action", "action", "initialize", "alloc", "match error", "ni prefix", prefix, "alloc prefix", alloc.Spec.Prefix)
						}
						_, err := r.AllocateIPPrefix(ctx, &alloc, false, nil)
						if err != nil {
							return err
						}
						/*
							a := netaddr.MustParseIPPrefix(prefix)
							route := table.NewRoute(a)
							route.UpdateLabel(labels)
							rt.Add(route)
						*/

						r.l.Info("ipam action", "action", "initialize", "added alloc", prefix)
					}
				}
			default:
			}
		}
	}
	return nil
}

func (r *ipam) Delete(crName string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.l = log.FromContext(context.Background())
	r.l.Info("ipam action", "action", "delete", "name", crName)
	delete(r.ipam, crName)
}

type AllocatedPrefix struct {
	AllocatedPrefix string
	Gateway         string
}

// AllocateIPPrefix allocates the prefix
// A prefix can be present or not
func (r *ipam) AllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation, dryrun bool, dryrunrt *table.RouteTable) (*AllocatedPrefix, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("allocate prefix ", "alloc", alloc)

	niName := types.NamespacedName{
		Namespace: alloc.GetNamespace(),
		Name:      alloc.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey],
	}

	var rt *table.RouteTable
	if dryrun {
		rt = dryrunrt
	} else {
		var err error
		rt, err = r.validateIPAllocation(niName)
		if err != nil {
			r.l.Error(err, "allocate prefix validation failed")
			return nil, err
		}
	}

	s, err := getSelectors(alloc)
	if err != nil {
		return nil, err
	}
	prefix := alloc.Spec.Prefix
	// The allocation request contains an ip prefix
	if prefix != "" {
		r.l.Info("allocate with prefix ", "prefix", prefix)
		a := netaddr.MustParseIPPrefix(prefix)
		// lookup the prefix in the routing table
		_, ok, err := rt.Get(a)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get ip prefix")
		}
		route := table.NewRoute(a)
		route.UpdateLabel(s.labels)

		if ok {
			// update the prefix to ensure we get the latest labels
			if err := rt.Update(route); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					return nil, errors.Wrap(err, "cannot update prefix")
				}
			}
			return &AllocatedPrefix{
				AllocatedPrefix: iputil.GetPrefixFromAlloc(prefix, alloc),
			}, r.updateNetworkInstanceStatus(ctx, niName, rt, dryrun)
		}
		// route does not exist
		if err := rt.Add(route); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "cannot add prefix")
			}
		}
		return &AllocatedPrefix{
			AllocatedPrefix: iputil.GetPrefixFromAlloc(prefix, alloc),
		}, r.updateNetworkInstanceStatus(ctx, niName, rt, dryrun)

	}
	// Allocation WITHOUT prefix
	r.l.Info("allocate w/o prefix ")
	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	routes := rt.GetByLabel(s.allocSelector)
	if len(routes) != 0 {
		route := routes[0]
		// there should only be 1 route with this name in the route table
		p := route.IPPrefix()
		prefixLength := iputil.GetPrefixLengthAsInt(route.IPPrefix())
		if prefixLength == 32 || prefixLength == 128 {
			ppl := s.labels[ipamv1alpha1.NephioParentPrefixLengthKey]
			net := s.labels[ipamv1alpha1.NephioNetworkKey]
			pfx := strings.Join([]string{net, ppl}, "/")
			if ppl != "" && net != "" {
				p = netaddr.MustParseIPPrefix(pfx)
			}
		}

		s.labels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(p))
		s.labels[ipamv1alpha1.NephioPrefixLengthKey] = string(iputil.GetAddressPrefixLength(p))
		s.labels[ipamv1alpha1.NephioPrefixKindKey] = string(ipamv1alpha1.PrefixKindNetwork)
		s.labels[ipamv1alpha1.NephioNetworkKey] = p.Masked().IP().String()
		route.UpdateLabel(s.labels)

		if err := rt.Update(route); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "route insertion failed")
			}
		}

		allocatedPrefix := &AllocatedPrefix{
			AllocatedPrefix: p.String(),
		}

		r.l.Info("gateway", "gatewaySelector", s.gatewaySelector)
		routes = rt.GetByLabel(s.gatewaySelector)
		if len(routes) > 0 {
			r.l.Info("gateway", "routes", routes)
			allocatedPrefix.Gateway = strings.Split(routes[0].IPPrefix().String(), "/")[0]
		}

		return allocatedPrefix, r.updateNetworkInstanceStatus(ctx, niName, rt, dryrun)

	}
	// NEW Allocation required
	r.l.Info("allocate w/o prefix", "labelSelector", s.labelSelector)
	routes = rt.GetByLabel(s.labelSelector)
	if len(routes) == 0 {
		return nil, fmt.Errorf("no available routes based on the label selector: %v", s.labelSelector)
	}
	// return /32 or /128 if no prefix length was supplied
	if alloc.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindNetwork) &&
		alloc.Spec.PrefixLength != 0 {
		return nil, fmt.Errorf("a prefix kind network w/o prefix cannot request a prefix with prefix length : %d", alloc.Spec.PrefixLength)
	}

	prefixLength := iputil.GetPrefixLengthFromAlloc(routes[0], alloc)
	selectedRoute := r.GetSelectedRouteWithPrefixLength(routes, alloc, s)
	if selectedRoute == nil {
		return nil, fmt.Errorf("no route found with requested prefixLength: %d", prefixLength)
	}

	a, ok := rt.FindFreePrefix(selectedRoute.IPPrefix(), prefixLength)
	if !ok {
		return nil, errors.New("no free prefix found")
	}
	route := table.NewRoute(a)
	prefix = a.String()
	if prefixLength == 32 || prefixLength == 128 {
		s.labels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.GetPrefixLength(selectedRoute.IPPrefix())
		n := strings.Split(a.String(), "/")
		prefix = strings.Join([]string{n[0], iputil.GetPrefixLength(selectedRoute.IPPrefix())}, "/")
	}
	s.labels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(selectedRoute.IPPrefix()))
	s.labels[ipamv1alpha1.NephioPrefixLengthKey] = string(iputil.GetAddressPrefixLength(a))
	s.labels[ipamv1alpha1.NephioPrefixKindKey] = alloc.Kind
	s.labels[ipamv1alpha1.NephioNetworkKey] = selectedRoute.IPPrefix().Masked().IP().String()
	route.UpdateLabel(s.labels)
	if err := rt.Update(route); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return nil, errors.Wrap(err, "route insertion failed")
		}
	}

	allocatedPrefix := &AllocatedPrefix{
		AllocatedPrefix: prefix,
	}

	r.l.Info("gateway", "gatewaySelector", s.gatewaySelector)
	routes = rt.GetByLabel(s.gatewaySelector)
	if len(routes) > 0 {
		r.l.Info("gateway", "routes", routes)
		allocatedPrefix.Gateway = strings.Split(routes[0].IPPrefix().String(), "/")[0]
	}

	r.l.Info("allocatedPrefix", "allocatedPrefix", allocatedPrefix)
	return allocatedPrefix, r.updateNetworkInstanceStatus(ctx, niName, rt, dryrun)

}

func (r *ipam) DeAllocateIPPrefixes(ctx context.Context, allocs ...*ipamv1alpha1.IPAllocation) error {
	r.l = log.FromContext(ctx)

	niName := types.NamespacedName{
		Namespace: allocs[0].GetNamespace(),
		Name:      allocs[0].Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey],
	}

	// we can  assume each alloc has the same network instance
	rt, err := r.validateIPAllocation(niName)
	if err != nil {
		return err
	}

	for _, alloc := range allocs {
		r.l.Info("deallocate prefix ", "alloc", alloc)

		s, err := getSelectors(alloc)
		if err != nil {
			return err
		}

		routes := rt.GetByLabel(s.allocSelector)
		for _, route := range routes {
			_, ok, err := rt.Delete(route)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("prefix not deleted")
			}
		}
	}
	return r.updateNetworkInstanceStatus(ctx, niName, rt, false)
}

func (r *ipam) IsLatestPrefixInNetwork(cr *ipamv1alpha1.IPPrefix) bool {
	//r.l = log.FromContext(ctx)
	niName := types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      cr.Spec.NetworkInstance,
	}
	rt, ok := r.Get(niName.String())
	if !ok {
		// THIS SHOULD NEVER OCCUR
		return false
	}

	// get all routes with the network label
	l, _ := GetNetworkLabelSelector(cr)
	routes := rt.GetByLabel(l)

	p := netaddr.MustParseIPPrefix(cr.Spec.Prefix)

	totalCount := len(routes)
	validCount := 0
	for _, route := range routes {
		switch route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey) {
		case cr.Name,
			p.Masked().IP().String(),
			strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-"):
			validCount++
		}
		r.l.Info("is latest prefix in network",
			"allocationName", route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey),
			"netName", strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-"),
			"totalCount", totalCount,
			"validCount", validCount,
		)
	}
	return totalCount == validCount
}

// validateIPAllocation validatess the network instance existance in the ipam
func (r *ipam) validateIPAllocation(niName types.NamespacedName) (*table.RouteTable, error) {
	rt, ok := r.Get(niName.String())
	if !ok {
		return nil, fmt.Errorf("ipam ni not ready or network-instance %s not correct", niName)
	}
	r.l.Info("validateIPAllocation", "ipam size", rt.Size())
	return rt, nil
}

func (r *ipam) updateNetworkInstanceStatus(ctx context.Context, niName types.NamespacedName, rt *table.RouteTable, dryrun bool) error {
	if dryrun {
		return nil
	}
	// update allocations based on latest routing table
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.c.Get(ctx, niName, ni); err != nil {
		return errors.Wrap(err, "cannot get network instance")
	}

	// always reinitialize the allocations based on latest info
	ni.Status.Allocations = make(map[string]labels.Set)
	for _, r := range rt.GetTable() {
		ni.Status.Allocations[r.String()] = *r.GetLabels()
	}
	return errors.Wrap(r.c.Status().Update(ctx, ni), "cannot update ni status")

}

func (r *ipam) GetSelectedRouteWithPrefixLength(routes table.Routes, alloc *ipamv1alpha1.IPAllocation, s *selectors) *table.Route {
	r.l.Info("alloc w/o prefix", "selector", s.labelSelector)
	r.l.Info("alloc w/o prefix", "routes", routes)
	prefixLength := iputil.GetPrefixLengthFromAlloc(routes[0], alloc)

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
