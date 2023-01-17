package ipam

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewAllocApplicator(c *ApplicatorConfig) Applicator {
	return &allocApplicator{
		alloc:        c.alloc,
		rib:          c.rib,
		watcher:      c.watcher,
		initializing: c.initializing,
	}
}

type allocApplicator struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	watcher      Watcher
	l            logr.Logger
}

func (r *allocApplicator) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "kind", r.alloc.GetPrefixKind())
	r.l.Info("dynamic allocation")

	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	route, ok, msg := r.getRoutesByOwner()
	if msg != "" {
		return nil, fmt.Errorf(msg)
	}
	if ok {
		r.l.Info("dynamic allocation: route exist")
		// route exists
		if err := r.updateRouteIfChanged(route); err != nil {
			return nil, err
		}

		// updates the status of the route
		// should not have changed -> allocatedPrefix + gateway allocs with prefixkind = network
		r.updateAllocStatus(route)

		r.l.Info("dynamic allocation: route exist", "allocatedPrefix", r.alloc.Status)
		return r.alloc, nil

	}
	r.l.Info("dynamic allocation: route does not exist, a new allocation is required")
	// A NEW Allocation is required
	// First allocate the routes based on the label selector
	routes := r.getRoutesByLabel()
	if len(routes) == 0 {
		return nil, fmt.Errorf("dynamic allocation: no available routes based on the selector labels %v", r.alloc.GetSelectorLabels())
	}

	// if the status indicated an allocated prefix, the client suggests to reallocate this prefix if possible
	if r.alloc.GetAllocatedPrefix() != "" {
		pi, err := iputil.New(r.alloc.GetAllocatedPrefix())
		if err != nil {
			return nil, err
		}
		r.l.Info("dynamic allocation, refresh allocated prefix",
			"allocatedPrefix", r.alloc.Status.AllocatedPrefix,
			"prefix", pi.GetIPPrefix(),
			"prefixlength", pi.GetPrefixLength())

		// check if the prefix is available
		p := r.rib.GetAvailablePrefixByBitLen(pi.GetIPPrefix(), uint8(pi.GetPrefixLength()))
		if p.IsValid() {
			// prefix is available -> select it and add the route to the rib
			if err := r.addSuggestedRoute(pi); err != nil {
				return nil, err
			}
			r.l.Info("dynamic allocation, refresh allocated prefix done",
				"allocatedPrefix", r.alloc.Status)
			// previously allocated prefix is available and reassigned
			return r.alloc, nil
		}
		r.l.Info("dynamic allocation refresh allocated prefix not available",
			"prefix", pi.GetIPPrefix(),
			"prefixlength", pi.GetPrefixLength())
	}

	// Second select routes based on prefix length
	// prefixlength is either set by the ipallocation request, if not it is derived from the
	// returned prefix and address family
	prefixLength := r.alloc.GetPrefixLengthFromRoute(routes[0])
	selectedRoute := r.GetSelectedRouteWithPrefixLength(routes, uint8(prefixLength.Int()))
	if selectedRoute == nil {
		return nil, fmt.Errorf("no route found with requested prefixLength: %d", prefixLength)
	}
	pi := iputil.NewPrefixInfo(selectedRoute.Prefix())
	r.l.Info("dynamic allocation new allocation", "selectedRoute", selectedRoute)
	p := r.rib.GetAvailablePrefixByBitLen(pi.GetIPPrefix(), uint8(prefixLength.Int()))
	if !p.IsValid() {
		return nil, errors.New("no free prefix found")
	}
	r.l.Info("dynamic allocation new allocation",
		"pi prefix", pi,
		"p prefix", p,
		"prefixLength", pi.GetPrefixLength(),
	)
	if err := r.addSelectedRoute(pi, p); err != nil {
		return nil, err
	}
	r.l.Info("dynamic allocation new allocation",
		"allocatedPrefix", r.alloc.Status)
	return r.alloc, nil
}

func (r *allocApplicator) GetGateway() string {
	gatewaySelector, err := r.alloc.GetGatewayLabelSelector()
	if err != nil {
		r.l.Error(err, "cannot get gateway label selector")
		return ""
	}
	r.l.Info("gateway", "gatewaySelector", gatewaySelector)
	routes := r.rib.GetByLabel(gatewaySelector)
	if len(routes) > 0 {
		r.l.Info("gateway", "routes", routes)
		return routes[0].Prefix().Addr().String()
	}
	return ""
}

func (r *allocApplicator) GetSelectedRouteWithPrefixLength(routes table.Routes, prefixLength uint8) *table.Route {
	r.l.Info("alloc w/o prefix", "routes", routes)

	if prefixLength == 32 || prefixLength == 128 {
		ownKindRoutes := make([]table.Route, 0)
		otherKindRoutes := make([]table.Route, 0)
		for _, route := range routes {
			switch r.alloc.GetPrefixKind() {
			case ipamv1alpha1.PrefixKindLoopback:
				if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] == string(r.alloc.GetPrefixKind()) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
			case ipamv1alpha1.PrefixKindNetwork:
				if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] == string(r.alloc.GetPrefixKind()) {
					ownKindRoutes = append(ownKindRoutes, route)
				}
				if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] == string(ipamv1alpha1.PrefixKindAggregate) {
					ownKindRoutes = append(otherKindRoutes, route)
				}
			case ipamv1alpha1.PrefixKindPool:
				if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] == string(r.alloc.GetPrefixKind()) {
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

func (r *allocApplicator) getRoutesByOwner() (table.Route, bool, string) {
	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	route := table.Route{}
	ownerSelector, err := r.alloc.GetOwnerSelector()
	if err != nil {
		return route, false, err.Error()
	}
	routes := r.rib.GetByLabel(ownerSelector)
	if len(routes) != 0 {
		if len(routes) > 1 {
			return route, false, fmt.Sprintf("multiple prefixes match the nsn labelselector, %v", routes)
		}
		// route found
		return routes[0], true, ""
	}
	// no route found
	return route, false, ""
}

func (r *allocApplicator) getRoutesByLabel() table.Routes {
	labelSelector, err := r.alloc.GetLabelSelector()
	if err != nil {
		r.l.Error(err, "cannot get label selector")
		return []table.Route{}
	}
	return r.rib.GetByLabel(labelSelector)
}

func (r *allocApplicator) updateRouteIfChanged(route table.Route) error {
	route = route.UpdateLabel(r.alloc.GetFullLabels())
	r.l.Info("dynamic allocation: route exist", "route", route)

	if !labels.Equals(r.alloc.GetFullLabels(), route.Labels()) {
		// update the route with the latest labels
		if err := r.rib.Set(route); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return errors.Wrap(err, "route insertion failed")
			}
		}
		// no need for informing clients as this is not a prefix
	}
	return nil
}

func (r *allocApplicator) addSuggestedRoute(pi iputil.PrefixInfo) error {
	// TODO if this is a create prefix and based on prefix kind we need to add all the contributing routes
	// like a mutate operation (look if we can add the mutate functionality here)

	// update the labels for the route that will be added in the routing table
	labels := r.alloc.GetFullLabels()
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(pi.GetAddressFamily())
	labels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(pi.GetIPAddress().BitLen())
	labels[ipamv1alpha1.NephioSubnetKey] = pi.GetSubnetName()
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pi.GetPrefixLength().String()
	}

	// we add the route based on the Address Prefix as thisis uniform with all prefixkinds
	// e.g. in network prefix we would have 10.0.0.10/24 presented to the user but in the rib
	// we allocate 10.0.0.10/32
	route := table.NewRoute(pi.GetIPAddressPrefix(), labels, map[string]any{})
	if err := r.rib.Add(route); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return errors.Wrap(err, "cannot add prefix")
		}
	}

	// update the gateway status, the allocatedPrefix is not changed so we dont update this
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		r.alloc.Status.Gateway = r.GetGateway()
	}
	return nil
}

func (r *allocApplicator) addSelectedRoute(pi iputil.PrefixInfo, p netip.Prefix) error {
	// since this is a dynamic allocation we dont know the prefix info ahead of time,
	// we need to augment the labels with the prefixlength info
	labels := r.alloc.GetFullLabels()
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(pi.GetAddressFamily())
	labels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(p.Bits())
	labels[ipamv1alpha1.NephioSubnetKey] = pi.GetSubnetName()
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pi.GetPrefixLength().String()
	}

	route := table.NewRoute(p, labels, map[string]any{})
	if err := r.rib.Add(route); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return errors.Wrap(err, "cannot add prefix")
		}
	}

	r.alloc.Status.AllocatedPrefix = p.String()
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		r.alloc.Status.Gateway = r.GetGateway()
		r.alloc.Status.AllocatedPrefix = netip.PrefixFrom(p.Addr(), pi.GetPrefixLength().Int()).String()
	}
	return nil
}

func (r *allocApplicator) updateAllocStatus(route table.Route) {
	r.alloc.Status.AllocatedPrefix = route.Prefix().String()

	// for prefixkind network we try to get the gateway
	// and we have to set the prefix using the parent prefix length
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork && !r.alloc.GetCreatePrefix() {
		r.alloc.Status.Gateway = r.GetGateway()

		parentRoutes := route.Parents(r.rib)
		r.l.Info("dynamic allocation: route exist", "parentRoutes", parentRoutes)
		r.alloc.Status.AllocatedPrefix = netip.PrefixFrom(route.Prefix().Addr(), parentRoutes[0].Prefix().Bits()).String()
	}
}
