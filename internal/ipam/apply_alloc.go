package ipam

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewAllocApplicator(c *ApplicatorConfig) Applicator {
	return &allocApplicator{
		alloc: c.alloc,
		rib:   c.rib,
	}
}

type allocApplicator struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	l     logr.Logger
}

func (r *allocApplicator) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "kind", r.alloc.GetPrefixKind())
	r.l.Info("apply allocation without prefix")

	// check if the prefix/alloc already exists in the routing table
	// based on the name of the allocator
	allocSelector, err := r.alloc.GetAllocSelector()
	if err != nil {
		return nil, err
	}
	routes := r.rib.GetByLabel(allocSelector)
	if len(routes) != 0 {
		if len(routes) > 1 {
			return nil, fmt.Errorf("multiple prefixes match the nsn labelselector, %v", routes)
		}
		// there should only be 1 route with this name in the route table
		route := routes[0]
		route = route.UpdateLabel(r.alloc.GetFullLabels())
		// update the route with the latest labels
		if err := r.rib.Add(route); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "route insertion failed")
			}
		}
		p := route.Prefix()
		r.alloc.Status.AllocatedPrefix = p.String()

		// for prefixkind network we try to get the gateway
		if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
			gatewaySelector, err := r.alloc.GetGatewayLabelSelector()
			if err != nil {
				return nil, err
			}
			r.l.Info("gateway", "gatewaySelector", gatewaySelector)
			routes = r.rib.GetByLabel(gatewaySelector)
			if len(routes) > 0 {
				r.l.Info("gateway", "routes", routes)
				r.alloc.Status.Gateway = routes[0].Prefix().Addr().String()
			}
		}
		r.l.Info("allocatedPrefix", "allocatedPrefix", r.alloc.Status)
		return r.alloc, nil

	}
	// A NEW Allocation is required
	labelSelector, err := r.alloc.GetLabelSelector()
	if err != nil {
		return nil, err
	}

	routes = r.rib.GetByLabel(labelSelector)
	if len(routes) == 0 {
		return nil, fmt.Errorf("no available routes based on the label selector: %v", labelSelector)
	}

	prefixLength := r.alloc.GetPrefixLengthFromRoute(routes[0])
	selectedRoute := r.GetSelectedRouteWithPrefixLength(routes, uint8(prefixLength.Int()))
	if selectedRoute == nil {
		return nil, fmt.Errorf("no route found with requested prefixLength: %d", prefixLength)
	}
	pi, err := iputil.New(selectedRoute.Prefix().String())
	if err != nil {
		return nil, err
	}

	p := r.rib.GetAvailablePrefixByBitLen(pi.GetIPPrefix(), uint8(prefixLength.Int()))
	if !p.IsValid() {
		return nil, errors.New("no free prefix found")
	}

	prefix := p.String()
	labels := r.alloc.GetFullLabels()
	// since this is a dynamic allocation we dont know the prefix info ahead of time,
	// we need to augment the labels with the prefixlength info
	if prefixLength == 32 || prefixLength == 128 {
		labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pi.GetPrefixLength().String()
		n := strings.Split(prefix, "/")
		prefix = strings.Join([]string{n[0], pi.GetPrefixLength().String()}, "/")
	}
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(pi.GetAddressFamily())
	labels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(p.Bits())
	labels[ipamv1alpha1.NephioSubnetKey] = pi.GetSubnetName()
	route := table.NewRoute(p, labels, map[string]any{})

	if err := r.rib.Add(route); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return nil, errors.Wrap(err, "cannot add prefix")
		}
	}

	r.alloc.Status.AllocatedPrefix = prefix

	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		gatewaySelector, err := r.alloc.GetGatewayLabelSelector()
		if err != nil {
			return nil, err
		}
		r.l.Info("gateway", "gatewaySelector", gatewaySelector)
		routes = r.rib.GetByLabel(gatewaySelector)
		if len(routes) > 0 {
			r.l.Info("gateway", "routes", routes)
			r.alloc.Status.Gateway = routes[0].Prefix().Addr().String()
		}
	}

	r.l.Info("allocatedPrefix", "allocatedPrefix", r.alloc.Status)
	return r.alloc, nil
}

func (r *allocApplicator) GetSelectedRouteWithPrefixLength(routes table.Routes, prefixLength uint8) *table.Route {
	r.l.Info("alloc w/o prefix", "routes", routes)

	if prefixLength == 32 || prefixLength == 128 {
		return &routes[0]
	} else {
		for _, route := range routes {
			if route.Prefix().Bits() < int(prefixLength) {
				return &route
			}
		}
	}
	return nil
}