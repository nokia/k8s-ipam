package prefix

import (
	"fmt"
	"sync"

	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/ipam"
	"github.com/henderiw-nephio/ipam/internal/utils/iputil"
	"inet.af/netaddr"
)

func initializePrefixValidators(ipam ipam.Ipam) PrefixValidators {
	v := NewPrefixValidators()

	networkPrefixValidator := NewPrefixValidator(&PrefixValidationConfig{
		Kind:               ipamv1alpha1.PrefixKindNetwork,
		Ipam:               ipam,
		IsAddressFn:        IsAddressGenericFn,
		IsAddressInNetFn:   IsAddressInNetNopFn,
		ExactPrefixMatchFn: ExactPrefixMatchNetworkFn,
		ChildrenExistFn:    ChildrenExistGenericFn,
		ParentExistFn:      ParentExistNetworkFn,
	})
	loopbackPrefixValidator := NewPrefixValidator(&PrefixValidationConfig{
		Kind:               ipamv1alpha1.PrefixKindLoopback,
		Ipam:               ipam,
		IsAddressFn:        IsAddressNopFn,
		IsAddressInNetFn:   IsAddressInNetGenericFn,
		ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
		ChildrenExistFn:    ChildrenExistLoopbackFn,
		ParentExistFn:      ParentExistLoopbackFn,
	})
	poolPrefixValidator := NewPrefixValidator(&PrefixValidationConfig{
		Kind:               ipamv1alpha1.PrefixKindPool,
		Ipam:               ipam,
		IsAddressFn:        IsAddressGenericFn,
		IsAddressInNetFn:   IsAddressInNetGenericFn,
		ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
		ChildrenExistFn:    ChildrenExistGenericFn,
		ParentExistFn:      ParentExistPoolFn,
	})
	aggregatePrefixValidator := NewPrefixValidator(&PrefixValidationConfig{
		Kind:               ipamv1alpha1.PrefixKindAggregate,
		Ipam:               ipam,
		IsAddressFn:        IsAddressGenericFn,
		IsAddressInNetFn:   IsAddressInNetGenericFn,
		ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
		ChildrenExistFn:    ChildrenExistNopFn,
		ParentExistFn:      ParentExistAggregateFn,
	})

	v.Register(ipamv1alpha1.PrefixKindNetwork, networkPrefixValidator)
	v.Register(ipamv1alpha1.PrefixKindLoopback, loopbackPrefixValidator)
	v.Register(ipamv1alpha1.PrefixKindPool, poolPrefixValidator)
	v.Register(ipamv1alpha1.PrefixKindAggregate, aggregatePrefixValidator)
	return v
}

type PrefixValidators interface {
	Register(ipamv1alpha1.PrefixKind, PrefixValidator)
	Get(ipamv1alpha1.PrefixKind) PrefixValidator
}

func NewPrefixValidators() PrefixValidators {
	return &prefixValidators{
		v: map[ipamv1alpha1.PrefixKind]PrefixValidator{},
	}
}

type prefixValidators struct {
	m sync.RWMutex
	v map[ipamv1alpha1.PrefixKind]PrefixValidator
}

func (r *prefixValidators) Register(k ipamv1alpha1.PrefixKind, v PrefixValidator) {
	r.m.Lock()
	defer r.m.Unlock()
	r.v[k] = v
}

func (r *prefixValidators) Get(k ipamv1alpha1.PrefixKind) PrefixValidator {
	r.m.Lock()
	defer r.m.Unlock()
	return r.v[k]
}

func IsAddressNopFn(kind ipamv1alpha1.PrefixKind, p netaddr.IPPrefix) string { return "" }

func IsAddressGenericFn(kind ipamv1alpha1.PrefixKind, p netaddr.IPPrefix) string {
	if iputil.IsAddress(p) {
		return fmt.Sprintf("a %s cannot be created with address (/32, /128) based prefixes, got %s", string(kind), p.String())
	}
	return ""
}

func IsAddressInNetNopFn(kind ipamv1alpha1.PrefixKind, p netaddr.IPPrefix) string { return "" }

func IsAddressInNetGenericFn(kind ipamv1alpha1.PrefixKind, p netaddr.IPPrefix) string {
	if p.IPNet().String() != p.Masked().String() {
		return fmt.Sprintf("a %s prefix cannot have net <> address", string(kind))
	}
	return ""
}

func ExactPrefixMatchNetworkFn(kind ipamv1alpha1.PrefixKind, route *table.Route, cr *ipamv1alpha1.IPPrefix) string {
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindNetwork) {
		return fmt.Sprintf("%s prefix in use by %s",
			string(kind),
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	if route.GetLabels().Get(ipamv1alpha1.NephioNetworkNameKey) != cr.Spec.Network {
		return fmt.Sprintf("%s prefix is matching the wrong network got %s, requested %s",
			string(kind),
			route.GetLabels().Get(ipamv1alpha1.NephioNetworkNameKey),
			cr.Spec.Network)
	}
	// all good, net exists already
	return ""
}

func ExactPrefixMatchGenericFn(kind ipamv1alpha1.PrefixKind, route *table.Route, cr *ipamv1alpha1.IPPrefix) string {
	if route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey) != cr.GetName() {
		return fmt.Sprintf("%s prefix in use by %s",
			string(kind),
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	// all good, net exists already
	return ""
}

func ChildrenExistNopFn(kind ipamv1alpha1.PrefixKind, route *table.Route) string { return "" }

func ChildrenExistLoopbackFn(kind ipamv1alpha1.PrefixKind, route *table.Route) string {
	return fmt.Sprintf("a more specific prefix was already allocated %s, loopbacks need to be created in hierarchy", route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
}

func ChildrenExistGenericFn(kind ipamv1alpha1.PrefixKind, route *table.Route) string {
	return fmt.Sprintf("a more specific prefix was already allocated %s, nesting not allowed for %s",
		route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey),
		string(kind),
	)
}

func ParentExistNetworkFn(kind ipamv1alpha1.PrefixKind, route *table.Route, p netaddr.IPPrefix) string {
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
		return fmt.Sprintf("nesting network prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s of kind %s",
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey),
			route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey),
		)
	}
	return ""
}

func ParentExistLoopbackFn(kind ipamv1alpha1.PrefixKind, route *table.Route, p netaddr.IPPrefix) string {
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

func ParentExistPoolFn(kind ipamv1alpha1.PrefixKind, route *table.Route, p netaddr.IPPrefix) string {
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

func ParentExistAggregateFn(kind ipamv1alpha1.PrefixKind, route *table.Route, p netaddr.IPPrefix) string {
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.GetLabels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
		return fmt.Sprintf("nesting aggregate prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s",
			route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey))
	}
	return ""
}
