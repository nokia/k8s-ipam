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

package prefix

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/ipam"
	"github.com/henderiw-nephio/ipam/internal/utils/iputil"
	"inet.af/netaddr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type IsAddressFn func(kind ipamv1alpha1.PrefixKind, p netaddr.IPPrefix) string
type IsAddressInNetFn func(kind ipamv1alpha1.PrefixKind, p netaddr.IPPrefix) string
type ExactPrefixMatchFn func(kind ipamv1alpha1.PrefixKind, route *table.Route, cr *ipamv1alpha1.IPPrefix) string
type ChildrenExistFn func(kind ipamv1alpha1.PrefixKind, route *table.Route) string
type ParentExistFn func(kind ipamv1alpha1.PrefixKind, route *table.Route, p netaddr.IPPrefix) string

type PrefixValidator interface {
	Validate(ctx context.Context, cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) (string, error)
}

type PrefixValidationConfig struct {
	Kind               ipamv1alpha1.PrefixKind
	Ipam               ipam.Ipam
	IsAddressFn        IsAddressFn
	IsAddressInNetFn   IsAddressInNetFn
	ExactPrefixMatchFn ExactPrefixMatchFn
	ChildrenExistFn    ChildrenExistFn
	ParentExistFn      ParentExistFn
}

type prefixvalidator struct {
	kind               ipamv1alpha1.PrefixKind
	ipam               ipam.Ipam
	isAddressFn        IsAddressFn
	isAddressInNetFn   IsAddressInNetFn
	exactPrefixMatchFn ExactPrefixMatchFn
	childrenExistFn    ChildrenExistFn
	parentExistFn      ParentExistFn

	l logr.Logger
}

func NewPrefixValidator(c *PrefixValidationConfig) PrefixValidator {
	return &prefixvalidator{
		kind:               c.Kind,
		ipam:               c.Ipam,
		isAddressFn:        c.IsAddressFn,
		isAddressInNetFn:   c.IsAddressInNetFn,
		exactPrefixMatchFn: c.ExactPrefixMatchFn,
		childrenExistFn:    c.ChildrenExistFn,
		parentExistFn:      c.ParentExistFn,
	}
}

func (r *prefixvalidator) Validate(ctx context.Context, cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) (string, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("validate prefix", "cr", cr.GetName(), "prefix", p.String())
	rt, ok := r.ipam.Get(types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      cr.Spec.NetworkInstance,
	}.String())
	if !ok {
		return "", fmt.Errorf("ipam ni not ready or network-instance %s not correct", cr.Spec.NetworkInstance)
	}

	// copy the routing table for validation
	newrt := table.NewRouteTable()
	for _, route := range rt.GetTable() {
		newrt.Add(route)
	}

	// This might need to be generalized
	if r.kind == ipamv1alpha1.PrefixKindNetwork {
		if cr.Spec.Network == "" {
			return "network prefix cannot have an empty network", nil
		}
	}

	if msg := r.isAddressFn(r.kind, p); msg != "" {
		return msg, nil
	}
	if msg := r.isAddressInNetFn(r.kind, p); msg != "" {
		return msg, nil
	}

	route, ok, err := newrt.Get(p)
	if err != nil {
		return "", err
	}
	if ok {
		return r.exactPrefixMatchFn(r.kind, route, cr), nil
	}
	// exact prefix does not exist, create it for validation
	if _, err := r.ipam.AllocateIPPrefix(ctx, buildNopAllocation(cr, p, map[string]string{
		ipamv1alpha1.NephioPrefixKindKey: string(r.kind),
	}, map[string]string{}), true, newrt); err != nil {
		return "", err
	}
	// get the route again and check for children
	route, _, err = newrt.Get(p)
	if err != nil {
		return "", err
	}
	routes := route.GetChildren(newrt)
	if len(routes) > 0 {
		r.l.Info("got children", "routes", routes)

		if msg := r.childrenExistFn(r.kind, routes[0]); msg != "" {
			/*
				r.ipam.DeAllocateIPPrefix(ctx, buildNopAllocation(cr, p, map[string]string{
					ipamv1alpha1.NephioPrefixKind: string(ipamv1alpha1.PrefixKindNetwork),
				}, map[string]string{}))
			*/
			return msg, nil
		}
	}
	routes = route.GetParents(newrt)
	for _, route := range routes {
		if msg := r.parentExistFn(r.kind, route, p); msg != "" {
			/*
				r.ipam.DeAllocateIPPrefix(ctx, buildNopAllocation(cr, p, map[string]string{
					ipamv1alpha1.NephioPrefixKind: string(ipamv1alpha1.PrefixKindNetwork),
				}, map[string]string{}))
			*/
			return msg, nil
		}
	}

	// check uniquess of the network name within the network-instance
	// a network needs to be globally unique
	// This might need to be generalized
	if r.kind == ipamv1alpha1.PrefixKindNetwork {
		l, err := ipam.GetNetworkLabelSelector(cr)
		if err != nil {
			return "", err
		}
		routes = newrt.GetByLabel(l)
		for _, route := range routes {
			net := route.GetLabels().Get(ipamv1alpha1.NephioNetworkKey)
			prefixLength := route.GetLabels().Get(ipamv1alpha1.NephioPrefixLengthKey)
			if route.GetLabels().Get(ipamv1alpha1.NephioParentPrefixLengthKey) != "" {
				prefixLength = route.GetLabels().Get(ipamv1alpha1.NephioParentPrefixLengthKey)
			}
			if strings.Join([]string{p.IP().String(), iputil.GetPrefixLength(p)}, "-") !=
				strings.Join([]string{net, prefixLength}, "-") {
				return fmt.Sprintf("network is not unique, network already exist on prefix %s, \n", route.String()), nil
			}
		}
	}

	return "", nil
}

func buildNopAllocation(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix, labels, selectorLabels map[string]string) *ipamv1alpha1.IPAllocation {
	labels[ipamv1alpha1.NephioIPPrefixNameKey] = cr.GetName()
	selectorLabels[ipamv1alpha1.NephioNetworkInstanceKey] = cr.Spec.NetworkInstance
	return &ipamv1alpha1.IPAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cr.GetNamespace(),
			Name:      strings.Join([]string{p.IP().String(), iputil.GetPrefixLength(p)}, "-"),
			Labels:    labels,
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			Prefix: p.String(),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}
}
