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
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *applicator) ApplyDynamic(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetName(), "kind", r.claim.Spec.Kind)
	r.l.Info("dynamic claim")

	// check if the prefix/claim already exists in the routing table
	// based on the name of the claim
	routes, err := r.getRoutesByOwner()
	if err != nil {
		return err
	}
	if len(routes) > 0 {
		r.l.Info("dynamic claim: route exist")
		// route exists
		if err := r.updateRib(ctx, routes); err != nil {
			return err
		}
		/*
			if err := r.updateRouteIfChanged(routes); err != nil {
				return err
			}

			// updates the status of the route
			// should not have changed -> claimed prefix + gateway claims with prefixkind = network
			r.updateClaimStatus(routes[0])
		*/

		r.l.Info("dynamic claim: route exist", "claimed prefix", r.claim.Status)
		return nil

	}
	r.l.Info("dynamic claim: route does not exist, a new claim is required")
	// A NEW Claim is required
	// First claim the routes based on the label selector
	rs := r.rib.GetTable()
	for _, route := range rs {
		r.l.Info("route in table", "route", route)
	}
	if r.claim.Spec.Selector != nil {
		r.l.Info("selector", "selector", r.claim.Spec.Selector.MatchLabels)
	} else {
		r.l.Info("no selector")
	}

	routes = r.getRoutesByLabel()
	if len(routes) == 0 {
		return fmt.Errorf("dynamic claim: no available routes based on the selector labels %v", r.claim.GetSelectorLabels())
	}

	// if the status indicated an claim prefix, the client suggests to reclaim this prefix if possible
	if r.claim.Status.Prefix != nil {
		pi, err := iputil.New(*r.claim.Status.Prefix)
		if err != nil {
			return err
		}
		r.l.Info("dynamic claim, refresh claim prefix",
			"claimPrefix", r.claim.Status.Prefix,
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
			r.l.Info("dynamic claim, refresh claim prefix done",
				"claimedPrefix", r.claim.Status)
			// previously claimed prefix is available and reassigned
			return nil
		}
		r.l.Info("dynamic claim refresh claim prefix not available",
			"prefix", pi.GetIPPrefix(),
			"prefixlength", pi.GetPrefixLength())
	}

	// Second select routes based on prefix length
	// prefixlength is either set by the claim request, if not it is derived from the
	// returned prefix and address family
	prefixLength := r.getPrefixLengthFromRoute(routes[0])
	selectedRoute := r.GetSelectedRouteWithPrefixLength(routes, uint8(prefixLength.Int()))
	if selectedRoute == nil {
		return fmt.Errorf("no route found with requested prefixLength: %d", prefixLength)
	}
	pi := iputil.NewPrefixInfo(selectedRoute.Prefix())
	r.l.Info("dynamic claim new claim", "selectedRoute", selectedRoute)
	p := r.rib.GetAvailablePrefixByBitLen(pi.GetIPPrefix(), uint8(prefixLength.Int()))
	if !p.IsValid() {
		return errors.New("no free prefix found")
	}
	r.l.Info("dynamic claim new claim",
		"pi prefix", pi,
		"p prefix", p,
		"prefixLength", pi.GetPrefixLength(),
	)
	r.pi = r.updatePrefixInfo(pi, p, prefixLength)
	if err := r.addRib(ctx); err != nil {
		return err
	}
	r.l.Info("dynamic claim new claim",
		"claimedPrefix", r.claim.Status)
	return nil
}

func (r *applicator) GetSelectedRouteWithPrefixLength(routes table.Routes, prefixLength uint8) *table.Route {
	r.l.Info("claim w/o prefix", "routes", routes)

	if prefixLength == 32 || prefixLength == 128 {
		ownKindRoutes := make([]table.Route, 0)
		otherKindRoutes := make([]table.Route, 0)
		for _, route := range routes {
			switch r.claim.Spec.Kind {
			case ipamv1alpha1.PrefixKindLoopback:
				if route.Labels()[resourcev1alpha1.NephioPrefixKindKey] == string(r.claim.Spec.Kind) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
				if route.Labels()[resourcev1alpha1.NephioPrefixKindKey] == string(ipamv1alpha1.PrefixKindAggregate) {
					otherKindRoutes = append(otherKindRoutes, route)
				}
			case ipamv1alpha1.PrefixKindNetwork:
				if route.Labels()[resourcev1alpha1.NephioPrefixKindKey] == string(r.claim.Spec.Kind) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
				if route.Labels()[resourcev1alpha1.NephioPrefixKindKey] == string(ipamv1alpha1.PrefixKindAggregate) {
					otherKindRoutes = append(otherKindRoutes, route)
				}
			case ipamv1alpha1.PrefixKindPool:
				if route.Labels()[resourcev1alpha1.NephioPrefixKindKey] == string(r.claim.Spec.Kind) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
			default:
				// aggregates with dynamic claim always have a prefix
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
	// check if the prefix/claim already exists in the routing table
	// based on the name of the claim
	//routes := []table.Route{}
	ownerSelector, err := r.claim.GetOwnerSelector()
	if err != nil {
		return []table.Route{}, err
	}
	routes := r.rib.GetByLabel(ownerSelector)
	if len(routes) != 0 {
		// for a prefixkind network with create prefix flag set it is possible that multiple
		// routes are returned since they were expanded
		if len(routes) > 1 && !(r.claim.Spec.CreatePrefix != nil && r.claim.Spec.Kind == ipamv1alpha1.PrefixKindNetwork) {
			return []table.Route{}, fmt.Errorf("multiple prefixes match the nsn labelselector, %v", routes)
		}
		// route found
		return routes, nil
	}
	// no route found
	return []table.Route{}, nil
}

func (r *applicator) getRoutesByLabel() table.Routes {
	labelSelector, err := r.claim.GetLabelSelector()
	if err != nil {
		r.l.Error(err, "cannot get label selector")
		return []table.Route{}
	}
	return r.rib.GetByLabel(labelSelector)
}

func (r *applicator) getPrefixLengthFromRoute(route table.Route) iputil.PrefixLength {
	if r.claim.Spec.PrefixLength != nil {
		return iputil.PrefixLength(*r.claim.Spec.PrefixLength)
	}
	return iputil.PrefixLength(route.Prefix().Addr().BitLen())
}

func (r *applicator) updatePrefixInfo(pi *iputil.Prefix, p netip.Prefix, prefixLength iputil.PrefixLength) *iputil.Prefix {
	if r.claim.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
		if r.claim.Spec.CreatePrefix != nil {

			return iputil.NewPrefixInfo(p)
		}
		return iputil.NewPrefixInfo(netip.PrefixFrom(p.Addr(), int(pi.GetPrefixLength())))
	}
	return iputil.NewPrefixInfo(netip.PrefixFrom(p.Addr(), prefixLength.Int()))

}
