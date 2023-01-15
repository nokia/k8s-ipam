package ipam

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Deletor interface {
	Delete(ctx context.Context) error
}

func NewDeleteApplicator(c *ApplicatorConfig) Deletor {
	return &applicator{
		initializing: c.initializing,
		alloc:        c.alloc,
		rib:          c.rib,
		watcher:      c.watcher,
	}
}

type applicator struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	watcher      Watcher
	l            logr.Logger
}

func (r *applicator) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "kind", r.alloc.GetPrefixKind())
	r.l.Info("delete")

	nsnSelector, err := r.alloc.GetNsnSelector()
	if err != nil {
		return err
	}
	r.l.Info("deallocate individual prefix", "nsnSelector", nsnSelector)

	routes := r.rib.GetByLabel(nsnSelector)
	for _, route := range routes {
		r.l = log.FromContext(ctx).WithValues("route prefix", route.Prefix())

		// this is a delete
		// only update when not initializing
		// only update when the prefix is a non /32 or /128
		// only update when the parent is a create prefix type
		if !r.initializing && !iputil.NewPrefixInfo(route.Prefix()).IsAddressPrefix() && r.alloc.GetCreatePrefix() {
			r.l.Info("route exists", "handle update for route", route, "labels", r.alloc.GetFullLabels())
			// delete the children from the rib
			// update the once that have a nsn different from the origin
			childRoutesToBeUpdated := []table.Route{}
			for _, childRoute := range route.Children(r.rib) {
				r.l.Info("route exists", "handle update for route", route, "child route", childRoute)
				if childRoute.Labels()[ipamv1alpha1.NephioNsnNameKey] != r.alloc.GetFullLabels()[ipamv1alpha1.NephioNsnNameKey] ||
					childRoute.Labels()[ipamv1alpha1.NephioNsnNamespaceKey] != r.alloc.GetFullLabels()[ipamv1alpha1.NephioNsnNamespaceKey] {
					childRoutesToBeUpdated = append(childRoutesToBeUpdated, childRoute)
					if err := r.rib.Delete(childRoute); err != nil {
						r.l.Error(err, "cannot delete route from rib", "route", childRoute)
					}
				}
			}
			// handler watch update to the source owner controller
			r.l.Info("route exists", "handle update for route", route, "child routes", childRoutesToBeUpdated)
			r.watcher.handleUpdate(ctx, childRoutesToBeUpdated, allocpb.StatusCode_Unknown)
		}
		/*
			r.l.Info("delete", "labels", route.Labels(), "data", route.GetData())
			if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] == string(ipamv1alpha1.PrefixKindNetwork) &&
				route.Labels()[ipamv1alpha1.NephioGvkKey] == ipamv1alpha1.OriginSystem {


				data := route.GetData()
				r.l.Info("delete", "data before", data)
				if data != nil {
					delete(data, r.alloc.GetFullLabels()[ipamv1alpha1.NephioIPContributingRouteKey])
					// if the data is not nil we update the route with the new data as there are still
					// contributing routes
					r.l.Info("delete", "data after", data)
					r.l.Info("deallocate individual prefix", "remaining data", data)
					if len(data) != 0 {
						// update the route
						route = route.SetData(data)
						if err := r.rib.Set(route); err != nil {
							r.l.Error(err, "cannot update prefix")
							if !strings.Contains(err.Error(), "already exists") {
								return errors.Wrap(err, "cannot update prefix")
							}
							return err
						}
						return nil
					}
				}
			}
		*/
		if err := r.rib.Delete(route); err != nil {
			return err
		}
		if !r.initializing {
			// handler watch update to the source owner controller
			r.watcher.handleUpdate(ctx, route.Children(r.rib), allocpb.StatusCode_Unknown)
		}
	}
	return nil
}
