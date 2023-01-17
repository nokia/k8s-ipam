package ipam

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Applicator interface {
	Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error)
}

type ApplicatorConfig struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	pi           iputil.PrefixInfo
	watcher      Watcher
}

func NewPrefixApplicator(c *ApplicatorConfig) Applicator {
	return &prefixApplicator{
		initializing: c.initializing,
		alloc:        c.alloc,
		rib:          c.rib,
		pi:           c.pi,
		watcher:      c.watcher,
	}
}

type prefixApplicator struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	pi           iputil.PrefixInfo
	watcher      Watcher
	l            logr.Logger
}

func (r *prefixApplicator) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "kind", r.alloc.GetPrefixKind(), "prefix", r.pi.GetIPPrefix().String(), "createPrefix", r.alloc.GetCreatePrefix())
	r.l.Info("prefix allocation")

	// get route
	var route table.Route
	var ok bool
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork && !r.alloc.GetCreatePrefix() {
		route, ok = r.rib.Get(r.pi.GetIPAddressPrefix())
	} else {
		route, ok = r.rib.Get(r.pi.GetIPPrefix())
	}

	// if route exists -> update
	// if route does not exist -> add
	if ok {
		r.l.Info("prefix allocation: route exists", "route", route)
		// prefix/route exists -> update

		if !labels.Equals(r.alloc.GetFullLabels(), route.Labels()) {
			route = route.UpdateLabel(r.alloc.GetFullLabels())
			r.l.Info("prefix allocation: route exists", "update route", route, "labels", r.alloc.GetFullLabels())
			if err := r.rib.Set(route); err != nil {
				r.l.Error(err, "cannot update prefix")
				if !strings.Contains(err.Error(), "already exists") {
					return nil, errors.Wrap(err, "cannot update prefix")
				}
			}
			// this is an update where the labels changed
			// only update when not initializing
			// only update when the prefix is a non /32 or /128
			if !r.initializing && !r.pi.IsAddressPrefix() && r.alloc.GetCreatePrefix() {
				r.l.Info("prefix allocation: route exists", "inform children of the change/update", route, "labels", r.alloc.GetFullLabels())
				// delete the children from the rib
				// update the once that have a nsn different from the origin
				childRoutesToBeUpdated := []table.Route{}
				for _, childRoute := range route.Children(r.rib) {
					r.l.Info("prefix allocation: route exists", "inform children of the change/update", route, "child route", childRoute)
					if childRoute.Labels()[ipamv1alpha1.NephioNsnNameKey] != r.alloc.GetFullLabels()[ipamv1alpha1.NephioNsnNameKey] ||
						childRoute.Labels()[ipamv1alpha1.NephioNsnNamespaceKey] != r.alloc.GetFullLabels()[ipamv1alpha1.NephioNsnNamespaceKey] {
						childRoutesToBeUpdated = append(childRoutesToBeUpdated, childRoute)
						if err := r.rib.Delete(childRoute); err != nil {
							r.l.Error(err, "cannot delete route from rib", "route", childRoute)
						}
					}
				}
				// handler watch update to the source owner controller
				r.l.Info("prefix allocation: route exists", "inform children of the change/update", route, "child routes", childRoutesToBeUpdated)
				r.watcher.handleUpdate(ctx, childRoutesToBeUpdated, allocpb.StatusCode_Unknown)
			}
		}

	} else {
		r.l.Info("prefix allocation: route does not exist", "route", route)
		// prefix does not exists -> add
		prefix := r.pi.GetIPPrefix()
		if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork && !r.alloc.GetCreatePrefix() {
			prefix = r.pi.GetIPAddressPrefix()
		}

		route := table.NewRoute(prefix, r.alloc.GetFullLabels(), map[string]any{})
		if err := r.rib.Add(route); err != nil {
			r.l.Error(err, "cannot add prefix")
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "cannot add prefix")
			}
		}
	}

	r.l.Info("prefix allocation", "allocated prefix", r.alloc.GetPrefix())
	//r.alloc.Status.AllocatedPrefix = r.alloc.GetPrefixFromNewAlloc()
	r.alloc.Status.AllocatedPrefix = r.alloc.GetPrefix()
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork && !r.alloc.GetCreatePrefix() {
		r.alloc.Status.Gateway = r.GetGateway()
	}
	return r.alloc, nil
}

func (r *prefixApplicator) GetGateway() string {
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
