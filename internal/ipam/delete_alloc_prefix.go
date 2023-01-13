package ipam

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Deletor interface {
	Delete(ctx context.Context) error
}

func NewDeleteApplicator(c *ApplicatorConfig) Deletor {
	return &applicator{
		alloc:   c.alloc,
		rib:     c.rib,
		watcher: c.watcher,
	}
}

type applicator struct {
	alloc   *ipamv1alpha1.IPAllocation
	rib     *table.RIB
	watcher Watcher
	l       logr.Logger
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
		if err := r.rib.Delete(route); err != nil {
			return err
		}
		// handle update to the owner of the object to indicate the routes has changed
		r.watcher.handleUpdate(ctx, route.Children(r.rib), allocpb.StatusCode_Unknown)
	}
	return nil
}
