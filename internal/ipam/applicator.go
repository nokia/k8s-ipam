/*
Copyright 2022 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ipam

import (
	"context"
	"net/netip"
	"strings"

	"github.com/hansthienpondt/nipam/pkg/table"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
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
	pi           *iputil.Prefix
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
	r.alloc.Status.Prefix = pointer.String(r.pi.GetIPPrefix().String())
	if r.alloc.Spec.Kind == ipamv1alpha1.PrefixKindNetwork && r.alloc.Spec.CreatePrefix == nil {
		r.alloc.Status.Gateway = pointer.String(r.getGateway())
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
			if r.pi != nil && !r.initializing && !r.pi.IsAddressPrefix() && r.alloc.Spec.CreatePrefix != nil {
				r.l.Info("prefix allocation: route exists", "inform children of the change/update", route, "labels", r.alloc.GetFullLabels())
				// delete the children from the rib
				// update the once that have a nsn different from the origin
				childRoutesToBeUpdated := []table.Route{}
				for _, childRoute := range route.Children(r.rib) {
					r.l.Info("prefix allocation: route exists", "inform children of the change/update", route, "child route", childRoute)
					if childRoute.Labels()[allocv1alpha1.NephioNsnNameKey] != r.alloc.GetFullLabels()[allocv1alpha1.NephioNsnNameKey] ||
						childRoute.Labels()[allocv1alpha1.NephioNsnNamespaceKey] != r.alloc.GetFullLabels()[allocv1alpha1.NephioNsnNamespaceKey] {
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
	r.alloc.Status.Prefix = pointer.String(routes[0].Prefix().String())
	if r.alloc.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
		if r.alloc.Spec.CreatePrefix == nil {
			r.alloc.Status.Gateway = pointer.String(r.getGateway())
		}
		if r.alloc.Spec.Prefix != nil {
			// we return the prefix we get in the request
			r.alloc.Status.Prefix = r.alloc.Spec.Prefix
		} else {
			// we return the parent prefix that was stored in the applicator context during the mutate label process
			if r.pi != nil {
				r.alloc.Status.Prefix = pointer.String(r.pi.GetIPPrefix().String())
				return nil
			}	
			r.alloc.Status.Prefix = pointer.String(routes[0].Prefix().String())
		}
	}
	return nil
}

func (r *applicator) getMutatedRoutesWithLabels() []table.Route {
	routes := []table.Route{}

	labels := r.alloc.GetUserDefinedLabels()
	labels[allocv1alpha1.NephioPrefixKindKey] = string(r.alloc.Spec.Kind)
	labels[allocv1alpha1.NephioAddressFamilyKey] = string(r.pi.GetAddressFamily())
	//labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetPrefixLength().String()
	labels[allocv1alpha1.NephioSubnetKey] = r.pi.GetSubnetName()

	prefix := r.pi.GetIPPrefix()

	if r.alloc.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
		if r.alloc.Spec.CreatePrefix != nil {
			switch {
			case r.pi.GetAddressFamily() == iputil.AddressFamilyIpv4 && r.pi.GetPrefixLength().Int() == 31,
				r.pi.GetAddressFamily() == iputil.AddressFamilyIpv6 && r.pi.GetPrefixLength().Int() == 127:
				routes = append(routes, r.mutateNetworkNetRoute(labels))
				//routes = append(routes, r.mutateNetworIPAddressRoute(labels))
			case r.pi.IsNorLastNorFirst():
				routes = append(routes, r.mutateNetworkNetRoute(labels))
				routes = append(routes, r.mutateNetworIPAddressRoute(labels))
				routes = append(routes, r.mutateNetworFirstAddressRoute(labels))
				routes = append(routes, r.mutateNetworLastAddressRoute(labels))
			case r.pi.IsFirst():
				routes = append(routes, r.mutateNetworkNetRoute(labels))
				routes = append(routes, r.mutateNetworIPAddressRoute(labels))
				routes = append(routes, r.mutateNetworLastAddressRoute(labels))
			case r.pi.IsLast():
				routes = append(routes, r.mutateNetworkNetRoute(labels))
				routes = append(routes, r.mutateNetworIPAddressRoute(labels))
				routes = append(routes, r.mutateNetworFirstAddressRoute(labels))
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
	delete(labels, allocv1alpha1.NephioGatewayKey)
	//labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetPrefixLength().String()
	return table.NewRoute(r.pi.GetIPSubnet(), labels, map[string]any{})
}

func (r *applicator) mutateNetworIPAddressRoute(l map[string]string) table.Route {
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	//labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetPrefixLength().String()
	return table.NewRoute(r.pi.GetIPAddressPrefix(), labels, map[string]any{})
}

func (r *applicator) mutateNetworFirstAddressRoute(l map[string]string) table.Route {
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	delete(labels, allocv1alpha1.NephioGatewayKey)
	//labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()
	return table.NewRoute(r.pi.GetFirstIPPrefix(), labels, map[string]any{})
}

func (r *applicator) mutateNetworLastAddressRoute(l map[string]string) table.Route {
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	delete(labels, allocv1alpha1.NephioGatewayKey)
	//labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()
	return table.NewRoute(r.pi.GetLastIPPrefix(), labels, map[string]any{})
}

func (r *applicator) GetUpdatedLabels(route table.Route) labels.Set {
	pi := iputil.NewPrefixInfo(route.Prefix())
	labels := r.alloc.GetUserDefinedLabels()
	labels[allocv1alpha1.NephioPrefixKindKey] = string(r.alloc.Spec.Kind)
	labels[allocv1alpha1.NephioAddressFamilyKey] = string(pi.GetAddressFamily())
	//labels[ipamv1alpha1.NephioPrefixLengthKey] = pi.GetPrefixLength().String()
	labels[allocv1alpha1.NephioSubnetKey] = pi.GetSubnetName()
	// for network based prefixes the prefixlength in the fib can be /32 but the representation
	// to the user is parent prefix based
	if r.alloc.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
		if pi.IsAddressPrefix() {
			parentRoutes := route.Parents(r.rib)
			//labels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.PrefixLength(parentRoutes[0].Prefix().Bits()).String()
			// construct the parent prefix info and store it in the applicator context
			r.pi = iputil.NewPrefixInfo(netip.PrefixFrom(pi.GetIPAddress(), parentRoutes[0].Prefix().Bits()))
			// delete the gateway if not the first or last address
			if pi.GetIPPrefix() == r.pi.GetFirstIPPrefix() || pi.GetIPPrefix() == r.pi.GetLastIPPrefix() {
				delete(labels, allocv1alpha1.NephioGatewayKey)
			}
			// overwirite the subnet key
			labels[allocv1alpha1.NephioSubnetKey] = r.pi.GetSubnetName()
		} else {
			// no gateway allowed for non address based prefixes
			delete(labels, allocv1alpha1.NephioGatewayKey)
			// overwirite the subnet key
			labels[allocv1alpha1.NephioSubnetKey] = pi.GetSubnetName()
			//labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pi.GetPrefixLength().String()
		}
	}

	// add ip pool labelKey if present
	if r.alloc.Spec.Kind == ipamv1alpha1.PrefixKindPool {
		labels[allocv1alpha1.NephioPoolKey] = "true"
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
