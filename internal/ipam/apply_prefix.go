package ipam

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewApplicator(c *ApplicatorConfig) Applicator {
	return &applicator{
		initializing: c.initializing,
		alloc:        c.alloc,
		rib:          c.rib,
		pi:           c.pi,
		watcher:      c.watcher,
	}
}

type applicator struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	pi           iputil.PrefixInfo
	watcher      Watcher
	l            logr.Logger
}

func (r *applicator) ApplyPrefix(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "kind", r.alloc.GetPrefixKind(), "prefix", r.pi.GetIPPrefix().String(), "createPrefix", r.alloc.GetCreatePrefix())
	r.l.Info("prefix allocation")

	// get route
	var route table.Route
	var ok bool
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		if !r.alloc.GetCreatePrefix() {
			route, ok = r.rib.Get(r.pi.GetIPAddressPrefix())
		} else {
			route, ok = r.rib.Get(r.pi.GetIPSubnet())
		}
	} else {
		route, ok = r.rib.Get(r.pi.GetIPPrefix())
	}

	// if route exists -> update
	// if route does not exist -> add
	if ok {
		// prefix/route exists -> update
		r.l.Info("prefix allocation: route exists", "route", route)

		// get all the routes from the routing table using the owner
		// to provide a common mechanism between dynamic allocation and prefix allocations
		routes, msg := r.getRoutesByOwner()
		if msg != "" {
			return fmt.Errorf(msg)
		}
		if err := r.updateRib(ctx, routes); err != nil {
			return err
		}
	} else {
		r.l.Info("prefix allocation: route does not exist", "route", route)

		// addRib mutates the routes and potentially expands the routes
		// add the routes to the RIB
		// update the status
		if err := r.addRib(ctx); err != nil {
			return err
		}
	}
	return nil
}
