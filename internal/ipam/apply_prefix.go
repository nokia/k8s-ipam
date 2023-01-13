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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Applicator interface {
	Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error)
}

type ApplicatorConfig struct {
	alloc   *ipamv1alpha1.IPAllocation
	rib     *table.RIB
	pi      iputil.PrefixInfo
	watcher Watcher
}

func NewPrefixApplicator(c *ApplicatorConfig) Applicator {
	return &prefixApplicator{
		alloc:   c.alloc,
		rib:     c.rib,
		pi:      c.pi,
		watcher: c.watcher,
	}
}

type prefixApplicator struct {
	alloc   *ipamv1alpha1.IPAllocation
	rib     *table.RIB
	pi      iputil.PrefixInfo
	watcher Watcher
	l       logr.Logger
}

func (r *prefixApplicator) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "kind", r.alloc.GetPrefixKind(), "prefix", r.pi.GetIPPrefix().String())
	r.l.Info("apply allocation with prefix")

	// get route
	route, ok := r.rib.Get(r.pi.GetIPPrefix())
	//if route exists -> update
	// if route does not exist -> add
	if ok {
		r.l.Info("route exists", "route", route)
		// prefix/route exists -> update

		// for prefixkind network and when this is a system route (subnet, first or last)
		// add the contributing route name to the data
		// replace the nsn with the replacement route name
		// delete the contributing and replacemnt keys
		labels := r.alloc.GetFullLabels()
		if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork &&
			r.alloc.GetFullLabels()[ipamv1alpha1.NephioGvkKey] == ipamv1alpha1.OriginSystem {

			data := route.GetData()
			labels, data = UpdateLabelsAndDataWithContributingRoutes(labels, data)

			route = route.SetData(data)
		}
		route = route.UpdateLabel(labels)
		r.l.Info("route exists", "update route", route, "labels", labels)
		if err := r.rib.Set(route); err != nil {
			r.l.Error(err, "cannot update prefix")
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "cannot update prefix")
			}
		}
		// handle update to the owner of the object to indicate the routes has changed
		r.watcher.handleUpdate(ctx, route.Children(r.rib), allocpb.StatusCode_Unknown)
	} else {
		r.l.Info("route does not exist", "route", route)
		// prefix does not exists -> add
		var route table.Route
		labels := r.alloc.GetFullLabels()
		if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork &&
			r.alloc.GetFullLabels()[ipamv1alpha1.NephioGvkKey] == ipamv1alpha1.OriginSystem {

			data := map[string]any{}
			labels, data = UpdateLabelsAndDataWithContributingRoutes(labels, data)

			route = table.NewRoute(r.pi.GetIPPrefix(), labels, data)
		} else {
			route = table.NewRoute(r.pi.GetIPPrefix(), labels, map[string]any{})
		}
		if err := r.rib.Add(route); err != nil {
			r.l.Error(err, "cannot add prefix")
			if !strings.Contains(err.Error(), "already exists") {
				return nil, errors.Wrap(err, "cannot add prefix")
			}
			r.watcher.handleUpdate(ctx, route.Children(r.rib), allocpb.StatusCode_Unknown)
		}
	}

	r.l.Info("prefix in alloc")
	r.alloc.Status.AllocatedPrefix = r.alloc.GetPrefixFromNewAlloc()
	return r.alloc, nil
}

// UpdateLabelsAndDataWithContributingRoutes updates the labels and data with the contributing route info and replacement route
func UpdateLabelsAndDataWithContributingRoutes(labels map[string]string, data map[string]any) (map[string]string, map[string]any) {
	// update data with contributing route
	if len(data) == 0 {
		data = map[string]any{}
	}
	data[labels[ipamv1alpha1.NephioIPContributingRouteKey]] = "dummy"

	// delete the contributing and replacement keys from labels
	delete(labels, ipamv1alpha1.NephioIPContributingRouteKey)
	//delete(labels, ipamv1alpha1.NephioReplacementNameKey)

	return labels, data
}
