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
	"net/netip"

	"github.com/hansthienpondt/nipam/pkg/table"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *applicator) ApplyAlloc(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "kind", r.alloc.Spec.Kind)
	r.l.Info("dynamic allocation")

	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	routes, err := r.getRoutesByOwner()
	if err != nil {
		return err
	}
	if len(routes) > 0 {
		r.l.Info("dynamic allocation: route exist")
		// route exists
		if err := r.updateRib(ctx, routes); err != nil {
			return err
		}
		/*
			if err := r.updateRouteIfChanged(routes); err != nil {
				return err
			}

			// updates the status of the route
			// should not have changed -> allocatedPrefix + gateway allocs with prefixkind = network
			r.updateAllocStatus(routes[0])
		*/

		r.l.Info("dynamic allocation: route exist", "allocatedPrefix", r.alloc.Status)
		return nil

	}
	r.l.Info("dynamic allocation: route does not exist, a new allocation is required")
	// A NEW Allocation is required
	// First allocate the routes based on the label selector
	rs := r.rib.GetTable()
	for _, route := range rs {
		r.l.Info("route in table", "route", route)
	}
	if r.alloc.Spec.Selector != nil {
		r.l.Info("selector", "selector", r.alloc.Spec.Selector.MatchLabels)
	} else {
		r.l.Info("no selector")
	}

	routes = r.getRoutesByLabel()
	if len(routes) == 0 {
		return fmt.Errorf("dynamic allocation: no available routes based on the selector labels %v", r.alloc.GetSelectorLabels())
	}

	// if the status indicated an allocated prefix, the client suggests to reallocate this prefix if possible
	if r.alloc.Status.Prefix != nil {
		pi, err := iputil.New(*r.alloc.Status.Prefix)
		if err != nil {
			return err
		}
		r.l.Info("dynamic allocation, refresh allocated prefix",
			"allocatedPrefix", r.alloc.Status.Prefix,
			"prefix", pi.GetIPPrefix(),
			"prefixlength", pi.GetPrefixLength())

		// check if the prefix is available
		p := r.rib.GetAvailablePrefixByBitLen(pi.GetIPPrefix(), uint8(pi.GetPrefixLength()))
		if p.IsValid() {
			// prefix is available -> select it and add the route to the rib
			r.pi = pi
			if err := r.addRib(ctx); err != nil {
				return err
			}
			r.l.Info("dynamic allocation, refresh allocated prefix done",
				"allocatedPrefix", r.alloc.Status)
			// previously allocated prefix is available and reassigned
			return nil
		}
		r.l.Info("dynamic allocation refresh allocated prefix not available",
			"prefix", pi.GetIPPrefix(),
			"prefixlength", pi.GetPrefixLength())
	}

	// Second select routes based on prefix length
	// prefixlength is either set by the ipallocation request, if not it is derived from the
	// returned prefix and address family
	prefixLength := r.getPrefixLengthFromRoute(routes[0])
	selectedRoute := r.GetSelectedRouteWithPrefixLength(routes, uint8(prefixLength.Int()))
	if selectedRoute == nil {
		return fmt.Errorf("no route found with requested prefixLength: %d", prefixLength)
	}
	pi := iputil.NewPrefixInfo(selectedRoute.Prefix())
	r.l.Info("dynamic allocation new allocation", "selectedRoute", selectedRoute)
	p := r.rib.GetAvailablePrefixByBitLen(pi.GetIPPrefix(), uint8(prefixLength.Int()))
	if !p.IsValid() {
		return errors.New("no free prefix found")
	}
	r.l.Info("dynamic allocation new allocation",
		"pi prefix", pi,
		"p prefix", p,
		"prefixLength", pi.GetPrefixLength(),
	)
	r.pi = r.updatePrefixInfo(pi, p, prefixLength)
	if err := r.addRib(ctx); err != nil {
		return err
	}
	r.l.Info("dynamic allocation new allocation",
		"allocatedPrefix", r.alloc.Status)
	return nil
}

func (r *applicator) GetSelectedRouteWithPrefixLength(routes table.Routes, prefixLength uint8) *table.Route {
	r.l.Info("alloc w/o prefix", "routes", routes)

	if prefixLength == 32 || prefixLength == 128 {
		ownKindRoutes := make([]table.Route, 0)
		otherKindRoutes := make([]table.Route, 0)
		for _, route := range routes {
			switch r.alloc.Spec.Kind {
			case ipamv1alpha1.PrefixKindLoopback:
				if route.Labels()[allocv1alpha1.NephioPrefixKindKey] == string(r.alloc.Spec.Kind) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
				if route.Labels()[allocv1alpha1.NephioPrefixKindKey] == string(ipamv1alpha1.PrefixKindAggregate) {
					otherKindRoutes = append(otherKindRoutes, route)
				}
			case ipamv1alpha1.PrefixKindNetwork:
				if route.Labels()[allocv1alpha1.NephioPrefixKindKey] == string(r.alloc.Spec.Kind) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
				if route.Labels()[allocv1alpha1.NephioPrefixKindKey] == string(ipamv1alpha1.PrefixKindAggregate) {
					otherKindRoutes = append(otherKindRoutes, route)
				}
			case ipamv1alpha1.PrefixKindPool:
				if route.Labels()[allocv1alpha1.NephioPrefixKindKey] == string(r.alloc.Spec.Kind) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
			default:
				// aggregates with dynamic allocations always have a prefix
			}
		}
		if len(ownKindRoutes) > 0 {
			return &ownKindRoutes[0]
		}
		if len(otherKindRoutes) > 0 {
			return &otherKindRoutes[0]
		}
		return nil
	} else {
		for _, route := range routes {
			if route.Prefix().Bits() < int(prefixLength) {
				return &route
			}
		}
	}
	return nil
}

func (r *applicator) getRoutesByOwner() (table.Routes, error) {
	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	//routes := []table.Route{}
	ownerSelector, err := r.alloc.GetOwnerSelector()
	if err != nil {
		return []table.Route{}, err
	}
	routes := r.rib.GetByLabel(ownerSelector)
	if len(routes) != 0 {
		// for a prefixkind network with create prefix flag set it is possible that multiple
		// routes are returned since they were expanded
		if len(routes) > 1 && !(r.alloc.Spec.CreatePrefix != nil && r.alloc.Spec.Kind == ipamv1alpha1.PrefixKindNetwork) {
			return []table.Route{}, fmt.Errorf("multiple prefixes match the nsn labelselector, %v", routes)
		}
		// route found
		return routes, nil
	}
	// no route found
	return []table.Route{}, nil
}

func (r *applicator) getRoutesByLabel() table.Routes {
	labelSelector, err := r.alloc.GetLabelSelector()
	if err != nil {
		r.l.Error(err, "cannot get label selector")
		return []table.Route{}
	}
	return r.rib.GetByLabel(labelSelector)
}

/*
func (r *applicator) updateRouteIfChanged(routes table.Routes) error {
	r.l.Info("dynamic allocation: route exist", "route", routes)

	for _, route := range routes {
		if !labels.Equals(r.alloc.GetSpecLabels(), route.Labels()) {
			route = route.UpdateLabel(r.alloc.GetSpecLabels())
			// update the route with the latest labels
			if err := r.rib.Set(route); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					return errors.Wrap(err, "route insertion failed")
				}
			}
			// no need for informing clients as this is not a prefix
		}
	}

	return nil
}
*/

func (r *applicator) getPrefixLengthFromRoute(route table.Route) iputil.PrefixLength {
	if r.alloc.Spec.PrefixLength != nil {
		return iputil.PrefixLength(*r.alloc.Spec.PrefixLength)
	}
	return iputil.PrefixLength(route.Prefix().Addr().BitLen())
}

func (r *applicator) updatePrefixInfo(pi *iputil.Prefix, p netip.Prefix, prefixLength iputil.PrefixLength) *iputil.Prefix {
	if r.alloc.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
		if r.alloc.Spec.CreatePrefix != nil {

			return iputil.NewPrefixInfo(p)
		}
		return iputil.NewPrefixInfo(netip.PrefixFrom(p.Addr(), int(pi.GetPrefixLength())))
	}
	return iputil.NewPrefixInfo(netip.PrefixFrom(p.Addr(), prefixLength.Int()))

}

/*
func (r *applicator) UpdateAllocStatus(route table.Route) {
	r.alloc.Status.AllocatedPrefix = route.Prefix().String()

	// for prefixkind network we try to get the gateway
	// and we have to set the prefix using the parent prefix length
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork && !r.alloc.GetCreatePrefix() {
		r.alloc.Status.Gateway = r.getGateway()

		parentRoutes := route.Parents(r.rib)
		r.l.Info("dynamic allocation: route exist", "parentRoutes", parentRoutes)
		r.alloc.Status.AllocatedPrefix = netip.PrefixFrom(route.Prefix().Addr(), parentRoutes[0].Prefix().Bits()).String()
	}
}
*/
