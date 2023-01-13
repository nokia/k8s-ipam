package ipam

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Deletor interface {
	Delete(ctx context.Context) error
}

func NewDeleteApplicator(c *ApplicatorConfig) Deletor {
	return &applicator{
		alloc: c.alloc,
		rib:   c.rib,
	}
}

type applicator struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	l     logr.Logger
}

func (r *applicator) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "kind", r.alloc.GetPrefixKind())
	r.l.Info("delete")

	allocSelector, err := r.alloc.GetAllocSelector()
	if err != nil {
		return err
	}
	r.l.Info("deallocate individual prefix", "allocSelector", allocSelector)

	routes := r.rib.GetByLabel(allocSelector)
	for _, route := range routes {
		if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork &&
			r.alloc.GetFullLabels()[ipamv1alpha1.NephioGvkKey] == ipamv1alpha1.OriginSystem {

			data := route.GetData()
			if data != nil {
				delete(data, r.alloc.GetFullLabels()[ipamv1alpha1.NephioIPContributingRouteKey])
				// if the data is not nil we update the route with the new data as there are still
				// contributing routes
				r.l.Info("deallocate individual prefix", "remaining data", data)
				if len(data) != 0 {
					// update the route
					route.SetData(data)
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
	}
	return nil
}
