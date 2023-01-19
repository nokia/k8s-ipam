package ipam

import (
	"context"
	"net/netip"
	"strings"

	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
)

type Applicator interface {
	ApplyPrefix(ctx context.Context) error
	ApplyAlloc(ctx context.Context) error
	Delete(ctx context.Context) error
}

type ApplicatorConfig struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	pi           iputil.PrefixInfo
	watcher      Watcher
}

func (r *applicator) addRib(ctx context.Context) error {
	// mutate prefixes -> expand and mutate object (return routes and labels)
	routes := r.getMutatedRoutesWithLabels()

	// range over the info and add the logic
	for _, route := range routes {
		if err := r.rib.Add(route); err != nil {
			r.l.Error(err, "cannot add prefix")
			if !strings.Contains(err.Error(), "already exists") {
				return errors.Wrap(err, "cannot add prefix")
			}
		}
	}
	// update status
	r.alloc.Status.AllocatedPrefix = r.pi.GetIPPrefix().String()
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork && !r.alloc.GetCreatePrefix() {
		r.alloc.Status.Gateway = r.getGateway()
	}
	return nil
}

func (r *applicator) updateRib(ctx context.Context, routes table.Routes) error {
	for _, route := range routes {
		// mutate the object
		lbls := r.GetUpdatedLabels(route)

		// check if the labels changed
		// if changed inform the owner GVKs through the watch
		if !labels.Equals(lbls, route.Labels()) {
			route = route.UpdateLabel(lbls)
			r.l.Info("update rib with new label info", "route prefix", route, "route labels", route.Labels(), "spec labels", lbls)
			if err := r.rib.Set(route); err != nil {
				r.l.Error(err, "cannot update prefix")
				if !strings.Contains(err.Error(), "already exists") {
					return errors.Wrap(err, "cannot update prefix")
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
	}
	// update the status
	r.alloc.Status.AllocatedPrefix = routes[0].Prefix().String()
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		if !r.alloc.GetCreatePrefix() {
			r.alloc.Status.Gateway = r.getGateway()
		}
		if r.alloc.GetPrefix() != "" {
			// we return the prefix we get in the request
			r.alloc.Status.AllocatedPrefix = r.alloc.GetPrefix()
		} else {
			// we return the parent prefix that was stored in the applicator context during the mutate label process
			r.alloc.Status.AllocatedPrefix = r.pi.GetIPPrefix().String()
		}
	}

	return nil
}

func (r *applicator) getMutatedRoutesWithLabels() []table.Route {
	routes := []table.Route{}

	labels := r.alloc.GetSpecLabels()
	labels[ipamv1alpha1.NephioPrefixKindKey] = string(r.alloc.GetPrefixKind())
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(r.pi.GetAddressFamily())
	labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetPrefixLength().String()

	prefix := r.pi.GetIPPrefix()

	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		if r.alloc.GetCreatePrefix() {
			// expand
			if r.pi.IsNorLastNorFirst() {
				routes = append(routes, r.mutateNetworkNetRoute(labels))
				routes = append(routes, r.mutateNetworIPAddressRoute(labels))
				routes = append(routes, r.mutateNetworFirstAddressRoute(labels))
				routes = append(routes, r.mutateNetworLastAddressRoute(labels))
			}
			if r.pi.IsFirst() {
				routes = append(routes, r.mutateNetworkNetRoute(labels))
				routes = append(routes, r.mutateNetworIPAddressRoute(labels))
				routes = append(routes, r.mutateNetworLastAddressRoute(labels))
			}
			if r.pi.IsLast() {
				routes = append(routes, r.mutateNetworkNetRoute(labels))
				routes = append(routes, r.mutateNetworIPAddressRoute(labels))
				routes = append(routes, r.mutateNetworFirstAddressRoute(labels))
				routes = append(routes, r.mutateNetworLastAddressRoute(labels))
			}
			return routes
		} else {
			// return address
			//labels[ipamv1alpha1.NephioParentPrefixLengthKey] = r.pi.GetPrefixLength().String()
			prefix = r.pi.GetIPAddressPrefix()
		}
	}
	routes = append(routes, table.NewRoute(prefix, labels, map[string]any{}))
	return routes
}

func (r *applicator) mutateNetworkNetRoute(l map[string]string) table.Route {
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	delete(labels, ipamv1alpha1.NephioGatewayKey)
	labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetPrefixLength().String()
	return table.NewRoute(r.pi.GetIPSubnet(), labels, map[string]any{})
}

func (r *applicator) mutateNetworIPAddressRoute(l map[string]string) table.Route {
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetPrefixLength().String()
	return table.NewRoute(r.pi.GetIPAddressPrefix(), labels, map[string]any{})
}

func (r *applicator) mutateNetworFirstAddressRoute(l map[string]string) table.Route {
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	delete(labels, ipamv1alpha1.NephioGatewayKey)
	labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()
	return table.NewRoute(r.pi.GetFirstIPPrefix(), labels, map[string]any{})
}

func (r *applicator) mutateNetworLastAddressRoute(l map[string]string) table.Route {
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	delete(labels, ipamv1alpha1.NephioGatewayKey)
	labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()
	return table.NewRoute(r.pi.GetLastIPPrefix(), labels, map[string]any{})
}

func (r *applicator) GetUpdatedLabels(route table.Route) labels.Set {
	pi := iputil.NewPrefixInfo(route.Prefix())
	labels := r.alloc.GetSpecLabels()
	labels[ipamv1alpha1.NephioPrefixKindKey] = string(r.alloc.GetPrefixKind())
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(pi.GetAddressFamily())
	labels[ipamv1alpha1.NephioPrefixLengthKey] = pi.GetPrefixLength().String()
	//labels[ipamv1alpha1.NephioSubnetKey] = r.pi.GetSubnetName()
	// for network based prefixes the prefixlength in the fib can be /32 but the representation
	// to the user is parent prefix based
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		if pi.IsAddressPrefix() {
			parentRoutes := route.Parents(r.rib)
			//labels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.PrefixLength(parentRoutes[0].Prefix().Bits()).String()
			// construct the parent prefix info and store it in the applicator context
			r.pi = iputil.NewPrefixInfo(netip.PrefixFrom(pi.GetIPAddress(), parentRoutes[0].Prefix().Bits()))
			// delete the gateway if not the first or last address
			if pi.GetIPPrefix() == r.pi.GetFirstIPPrefix() || pi.GetIPPrefix() == r.pi.GetLastIPPrefix() {
				delete(labels, ipamv1alpha1.NephioGatewayKey)
			}
		} else {
			// no gateway allowed for non address based prefixes
			delete(labels, ipamv1alpha1.NephioGatewayKey)
			//labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pi.GetPrefixLength().String()
		}
	}

	// add ip pool labelKey if present
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
		labels[ipamv1alpha1.NephioPoolKey] = "true"
	}
	return labels
}

func (r *applicator) getGateway() string {
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
