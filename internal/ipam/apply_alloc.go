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
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "kind", r.alloc.GetPrefixKind())
	r.l.Info("dynamic allocation")

	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	nsnSelector, err := r.alloc.GetNsnSelector()
	if err != nil {
		return nil, err
	}
	routes := r.rib.GetByLabel(nsnSelector)
	if len(routes) != 0 {
		if len(routes) > 1 {
			return nil, fmt.Errorf("multiple prefixes match the nsn labelselector, %v", routes)
		}

		// there should only be 1 route with this name in the route table
		route := routes[0]
		route = route.UpdateLabel(r.alloc.GetFullLabels())
		r.l.Info("dynamic allocation: route exist", "route", route)

		if !labels.Equals(r.alloc.GetFullLabels(), route.Labels()) {
			// update the route with the latest labels
			if err := r.rib.Set(route); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					return nil, errors.Wrap(err, "route insertion failed")
				}
			}
			// no need for informing clients as this is not a prefix
		}

		r.alloc.Status.AllocatedPrefix = route.Prefix().String()

		// for prefixkind network we try to get the gateway
		// and we have to set the prefix using the parent prefix length
		if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
			r.alloc.Status.Gateway = r.GetGateway()

			parentRoutes := route.Parents(r.rib)
			r.l.Info("dynamic allocation: route exist", "parentRoutes", parentRoutes)
			//parenPrefixLength, _ := strconv.Atoi(route.Labels()[ipamv1alpha1.NephioParentPrefixLengthKey])
			r.alloc.Status.AllocatedPrefix = netip.PrefixFrom(route.Prefix().Addr(), parentRoutes[0].Prefix().Bits()).String()

		}
		r.l.Info("dynamic allocation: route exist", "allocatedPrefix", r.alloc.Status)
		return r.alloc, nil

	}
	// A NEW Allocation is required
	// First allocate the routes based on the label selector
	labelSelector, err := r.alloc.GetLabelSelector()
	if err != nil {
		return nil, err
	}
	routes = r.rib.GetByLabel(labelSelector)
	if len(routes) == 0 {
		return nil, fmt.Errorf("no available routes based on the label selector: %v", labelSelector)
	}

	// check if the allocated prefix is available
	if r.alloc.Status.AllocatedPrefix != "" {
		pi, err := iputil.New(r.alloc.Status.AllocatedPrefix)
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
			// prefix is available
			labels := r.alloc.GetFullLabels()
			labels[ipamv1alpha1.NephioAddressFamilyKey] = string(pi.GetAddressFamily())
			labels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(pi.GetIPAddress().BitLen())
			if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
				labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pi.GetPrefixLength().String()
				labels[ipamv1alpha1.NephioSubnetKey] = pi.GetSubnetName()
			}

			route := table.NewRoute(pi.GetIPAddressPrefix(), labels, map[string]any{})
			if err := r.rib.Add(route); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					return nil, errors.Wrap(err, "cannot add prefix")
				}
			}

			if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
				r.alloc.Status.Gateway = r.GetGateway()
			}
			r.l.Info("allocatedPrefix", "allocatedPrefix", r.alloc.Status)
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
	r.l.Info("dynamic allocation", "selectedRoute", selectedRoute)
	p := r.rib.GetAvailablePrefixByBitLen(pi.GetIPPrefix(), uint8(prefixLength.Int()))
	if !p.IsValid() {
		return nil, errors.New("no free prefix found")
	}
	r.l.Info("dynamic allocation", "prefix", p)

	prefix := p.String()
	labels := r.alloc.GetFullLabels()
	// since this is a dynamic allocation we dont know the prefix info ahead of time,
	// we need to augment the labels with the prefixlength info
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pi.GetPrefixLength().String()
	}
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(pi.GetAddressFamily())
	labels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(p.Bits())
	labels[ipamv1alpha1.NephioSubnetKey] = pi.GetSubnetName()
	route := table.NewRoute(p, labels, map[string]any{})

	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		prefix = netip.PrefixFrom(p.Addr(), pi.GetPrefixLength().Int()).String()
	}

	r.l.Info("dynamic allocation",
		"pi prefix", pi,
		"p prefix", p,
		"prefix", prefix,
		"prefixLength", pi.GetPrefixLength(),
		"labels", labels)

	if err := r.rib.Add(route); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return nil, errors.Wrap(err, "cannot add prefix")
		}
	}
	/*
		if !r.initializing {
			// handler watch update to the source owner controller
			r.watcher.handleUpdate(ctx, route.Children(r.rib), allocpb.StatusCode_Unknown)
		}
	*/

	r.alloc.Status.AllocatedPrefix = prefix
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		r.alloc.Status.Gateway = r.GetGateway()
	}

	r.l.Info("allocatedPrefix", "allocatedPrefix", r.alloc.Status)
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
