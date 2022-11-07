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
	"fmt"
	"strings"

	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *ipam) validate(ctx context.Context, alloc *Allocation) (string, error) {
	r.vm.Lock()
	validateFnCfg := r.validator[ipamUsage{PrefixKind: ipamv1alpha1.PrefixKind(alloc.PrefixKind), HasPrefix: alloc.Prefix != ""}]
	r.vm.Unlock()

	if alloc.Prefix != "" {
		return r.validatePrefix(ctx, alloc, validateFnCfg)
	}
	return r.validateAlloc(ctx, alloc, validateFnCfg)
}

type ValidateInputFn func(alloc *Allocation) string
type IsAddressFn func(alloc *Allocation) string
type IsAddressInNetFn func(alloc *Allocation) string
type ExactPrefixMatchFn func(alloc *Allocation, route *table.Route) string
type ChildrenExistFn func(alloc *Allocation, route *table.Route) string
type ParentExistFn func(alloc *Allocation, route *table.Route) string
type FinalValidationFn func(alloc *Allocation, dryrunrt *table.RouteTable) string

type ValidationConfig struct {
	ValidateInputFn    ValidateInputFn
	IsAddressFn        IsAddressFn
	IsAddressInNetFn   IsAddressInNetFn
	ExactPrefixMatchFn ExactPrefixMatchFn
	ChildrenExistFn    ChildrenExistFn
	ParentExistFn      ParentExistFn
	FinalValidationFn  FinalValidationFn
}

func (r *ipam) validatePrefix(ctx context.Context, alloc *Allocation, fnc *ValidationConfig) (string, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("validate prefix", "cr", alloc.GetName(), "prefix", alloc.GetPrefix())

	dryrunrt, err := r.getRoutingTable(alloc, true)
	if err != nil {
		return "", err
	}

	if msg := fnc.ValidateInputFn(alloc); msg != "" {
		return msg, nil
	}
	if msg := fnc.IsAddressFn(alloc); msg != "" {
		return msg, nil
	}
	if msg := fnc.IsAddressInNetFn(alloc); msg != "" {
		return msg, nil
	}
	route, ok, err := dryrunrt.Get(alloc.GetIPPrefix())
	if err != nil {
		return "", err
	}
	if ok {
		return fnc.ExactPrefixMatchFn(alloc, route), nil
	}
	// exact prefix does not exist, create it for validation
	p := alloc.GetIPPrefix()
	route = table.NewRoute(p)
	route.UpdateLabel(map[string]string{
		ipamv1alpha1.NephioOriginKey:            "dummy",
		ipamv1alpha1.NephioPrefixKindKey:        string(alloc.PrefixKind),
		ipamv1alpha1.NephioIPPrefixNameKey:      alloc.GetName(),
		ipamv1alpha1.NephioNetworkInstanceKey:   alloc.GetNetworkInstance(),
		ipamv1alpha1.NephioIPAllocactionNameKey: strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-"),
		ipamv1alpha1.NephioPrefixLengthKey:      iputil.GetPrefixLength(p),
		ipamv1alpha1.NephioNetworkNameKey:       alloc.Network,
		ipamv1alpha1.NephioNetworkKey:           p.Masked().IP().String(),
	})
	if err := dryrunrt.Add(route); err != nil {
		return "", err
	}
	// get the route again and check for children
	route, _, err = dryrunrt.Get(p)
	if err != nil {
		return "", err
	}
	routes := route.GetChildren(dryrunrt)
	if len(routes) > 0 {
		r.l.Info("got children", "routes", routes)

		if msg := fnc.ChildrenExistFn(alloc, routes[0]); msg != "" {
			return msg, nil
		}
	}
	routes = route.GetParents(dryrunrt)
	for _, route := range routes {
		if msg := fnc.ParentExistFn(alloc, route); msg != "" {
			return msg, nil
		}
	}
	if msg := fnc.FinalValidationFn(alloc, dryrunrt); msg != "" {
		return msg, nil
	}
	return "", nil
}

func (r *ipam) validateAlloc(ctx context.Context, alloc *Allocation, fnc *ValidationConfig) (string, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("validate w/o prefix", "cr", alloc.GetName(), "prefix", alloc.GetPrefix())

	if msg := fnc.ValidateInputFn(alloc); msg != "" {
		return msg, nil
	}

	return "", nil
}

func ValidateInputNopFn(alloc *Allocation) string { return "" }

func ValidateInputNetworkWithPrefixFn(alloc *Allocation) string {
	// This might need to be generalized
	if alloc.Network == "" {
		return "network prefix cannot have an empty network"
	}

	return ""
}

func ValidateInputNetworkWithoutPrefixFn(alloc *Allocation) string {
	// This might need to be generalized
	if alloc.Network == "" {
		return "network prefix cannot have an empty network"
	}

	if alloc.PrefixLength != 0 {
		return fmt.Sprintf("a network prefix w/o prefix cannot request a prefix with prefix length : %d", alloc.PrefixLength)
	}
	return ""
}

func IsAddressNopFn(alloc *Allocation) string { return "" }

func IsAddressGenericFn(alloc *Allocation) string {
	p := alloc.GetIPPrefix()
	if iputil.IsAddress(p) {
		return fmt.Sprintf("a %s cannot be created with address (/32, /128) based prefixes, got %s", alloc.GetPrefixKind(), p.String())
	}
	return ""
}

func IsAddressInNetNopFn(alloc *Allocation) string { return "" }

func IsAddressInNetGenericFn(alloc *Allocation) string {
	p := alloc.GetIPPrefix()
	if p.IPNet().String() != p.Masked().String() {
		return fmt.Sprintf("a %s prefix cannot have net <> address", alloc.GetPrefixKind())
	}
	return ""
}

func ExactPrefixMatchNetworkFn(alloc *Allocation, route *table.Route) string {
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindNetwork) {
		return fmt.Sprintf("%s prefix in use by %s",
			alloc.GetPrefixKind(),
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	if route.GetLabels().Get(ipamv1alpha1.NephioNetworkNameKey) != alloc.GetNetwork() {
		return fmt.Sprintf("%s prefix is matching the wrong network got %s, requested %s",
			alloc.GetPrefixKind(),
			route.GetLabels().Get(ipamv1alpha1.NephioNetworkNameKey),
			alloc.GetNetwork())
	}
	// all good, net exists already
	return ""
}

func ExactPrefixMatchGenericFn(alloc *Allocation, route *table.Route) string {
	if route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey) != alloc.GetName() {
		return fmt.Sprintf("%s prefix in use by %s",
			alloc.GetPrefixKind(),
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	// all good, net exists already
	return ""
}

func ChildrenExistNopFn(alloc *Allocation, route *table.Route) string { return "" }

func ChildrenExistLoopbackFn(alloc *Allocation, route *table.Route) string {
	return fmt.Sprintf("a more specific prefix was already allocated %s, loopbacks need to be created in hierarchy",
		route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
}

func ChildrenExistGenericFn(alloc *Allocation, route *table.Route) string {
	return fmt.Sprintf("a more specific prefix was already allocated %s, nesting not allowed for %s",
		route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey),
		alloc.GetPrefixKind(),
	)
}

func ParentExistNetworkFn(alloc *Allocation, route *table.Route) string {
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
		return fmt.Sprintf("nesting network prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s of kind %s",
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey),
			route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey),
		)
	}
	return ""
}

func ParentExistLoopbackFn(alloc *Allocation, route *table.Route) string {
	p := alloc.GetIPPrefix()
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) ||
		route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindLoopback) {
		return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/loopback prefix is not allowed, prefix nested with %s",
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindLoopback) && !iputil.IsAddress(p) {
		return fmt.Sprintf("nesting loopback prefixes only possible with address (/32, /128) based prefixes, got %s", p.String())
	}
	return ""
}

func ParentExistPoolFn(alloc *Allocation, route *table.Route) string {
	p := alloc.GetIPPrefix()
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) ||
		route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindPool) {
		return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/pool prefix is not allowed, prefix nested with %s",
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindLoopback) && !iputil.IsAddress(p) {
		return fmt.Sprintf("nesting loopback prefixes only possible with address (/32, /128) based prefixes, got %s", p.String())
	}
	return ""
}

func ParentExistAggregateFn(alloc *Allocation, route *table.Route) string {
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
		return fmt.Sprintf("nesting aggregate prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s",
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	return ""
}

func FinalValidationNopFn(alloc *Allocation, dryrunrt *table.RouteTable) string { return "" }

func FinalValidationNetworkFn(alloc *Allocation, dryrunrt *table.RouteTable) string {
	l, err := alloc.GetNetworkLabelSelector()
	if err != nil {
		return err.Error()
	}
	routes := dryrunrt.GetByLabel(l)
	for _, route := range routes {
		net := route.GetLabels().Get(ipamv1alpha1.NephioNetworkKey)
		prefixLength := route.GetLabels().Get(ipamv1alpha1.NephioPrefixLengthKey)
		if route.GetLabels().Get(ipamv1alpha1.NephioParentPrefixLengthKey) != "" {
			prefixLength = route.GetLabels().Get(ipamv1alpha1.NephioParentPrefixLengthKey)
		}
		if strings.Join([]string{alloc.GetIPPrefix().IP().String(), iputil.GetPrefixLength(alloc.GetIPPrefix())}, "-") !=
			strings.Join([]string{net, prefixLength}, "-") {
			return fmt.Sprintf("network is not unique, network already exist on prefix %s, \n", route.String())
		}
	}
	return ""
}
