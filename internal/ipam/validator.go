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

/*
import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	//"github.com/hansthienpondt/goipam/pkg/table"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *ipam) validate(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) (string, error) {
	r.vm.Lock()
	validateFnCfg := r.validator[ipamUsage{PrefixKind: alloc.GetPrefixKind(), HasPrefix: alloc.GetPrefix() != ""}]
	r.vm.Unlock()

	if alloc.GetPrefix() != "" {
		return r.validatePrefix(ctx, alloc, validateFnCfg)
	}
	return r.validateAlloc(ctx, alloc, validateFnCfg)
}

type ValidateInputFn func(alloc *ipamv1alpha1.IPAllocation) string
type IsAddressFn func(alloc *ipamv1alpha1.IPAllocation) string
type IsAddressInNetFn func(alloc *ipamv1alpha1.IPAllocation) string
type ExactMatchPrefixFn func(alloc *ipamv1alpha1.IPAllocation) netip.Prefix
type ExactPrefixMatchFn func(alloc *ipamv1alpha1.IPAllocation, route table.Route) string
type ChildrenExistFn func(alloc *ipamv1alpha1.IPAllocation, route table.Route) string
type NoParentExistFn func(alloc *ipamv1alpha1.IPAllocation) string
type ParentExistFn func(alloc *ipamv1alpha1.IPAllocation, route table.Route) string
type FinalValidationFn func(alloc *ipamv1alpha1.IPAllocation, dryrunrt *table.RIB) string

type ValidationConfig struct {
	ValidateInputFn    ValidateInputFn
	IsAddressFn        IsAddressFn
	IsAddressInNetFn   IsAddressInNetFn
	ExactMatchPrefixFn ExactMatchPrefixFn
	ExactPrefixMatchFn ExactPrefixMatchFn
	ChildrenExistFn    ChildrenExistFn
	NoParentExistFn    NoParentExistFn
	ParentExistFn      ParentExistFn
	FinalValidationFn  FinalValidationFn
}

func (r *ipam) validatePrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation, fnc *ValidationConfig) (string, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("validate prefix", "cr", alloc.GetName(), "prefix", alloc.GetPrefix())

	dryrunRib, err := r.getRIB(alloc.GetNetworkInstance(), true, false)
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
	exactMatchPrefix := iputil.New(alloc.GetPrefix()).GetIPSubnet()
	if fnc.ExactMatchPrefixFn != nil {
		exactMatchPrefix = fnc.ExactMatchPrefixFn(alloc)
	}
	r.l.Info("validate prefix", "cr", alloc.GetName(), "ip prefix", iputil.New(alloc.GetPrefix()).GetIPPrefix())
	route, ok := dryrunRib.Get(exactMatchPrefix)
	r.l.Info("validate prefix exact route", "cr", alloc.GetName(), "route", route)
	if ok {
		return fnc.ExactPrefixMatchFn(alloc, route), nil
	}
	// exact prefix does not exist, create it for validation
	pi := iputil.New(alloc.GetPrefix())
	route = table.NewRoute(pi.GetIPSubnet(), map[string]string{
		ipamv1alpha1.NephioOwnerGvkKey:   "dummy",
		ipamv1alpha1.NephioOwnerNsnKey:   "dummy",
		ipamv1alpha1.NephioGvkKey:        ipamv1alpha1.IPAllocationKindGVKString,
		ipamv1alpha1.NephioNsnKey:        strings.ReplaceAll(types.NamespacedName{Name: alloc.GetName(), Namespace: alloc.GetNamespace()}.String(), "/", "-"),
		ipamv1alpha1.NephioPrefixKindKey: string(alloc.GetPrefixKind()),
		//ipamv1alpha1.NephioIPAllocactionNameKey: pi.GetSubnetName(),
		ipamv1alpha1.NephioPrefixLengthKey: pi.GetPrefixLength().String(),
		ipamv1alpha1.NephioSubnetKey:       pi.GetSubnetName(),
	}, map[string]any{})

	if err := dryrunRib.Add(route); err != nil {
		return "", err
	}

	for _, route := range dryrunRib.GetTable() {
		r.l.Info("got routes", "route", route)
	}
	// get the route again and check for children
	route, ok = dryrunRib.Get(pi.GetIPSubnet())
	if !ok {
		return "", fmt.Errorf("route just added, but a new get does not find it")
	}
	routes := route.Children(dryrunRib)
	if len(routes) > 0 {
		r.l.Info("got children", "routes", routes)

		if msg := fnc.ChildrenExistFn(alloc, routes[0]); msg != "" {
			return msg, nil
		}
	}
	routes = route.Parents(dryrunRib)
	if len(routes) == 0 {
		if msg := fnc.NoParentExistFn(alloc); msg != "" {
			return msg, nil
		}
	}
	for _, route := range routes {
		if msg := fnc.ParentExistFn(alloc, route); msg != "" {
			return msg, nil
		}
	}
	r.l.Info("validate prefix", "cr", alloc.GetName(), "prefix", alloc.GetPrefix(), "routes", routes)
	if msg := fnc.FinalValidationFn(alloc, dryrunRib); msg != "" {
		return msg, nil
	}
	return "", nil
}

func (r *ipam) validateAlloc(ctx context.Context, alloc *ipamv1alpha1.IPAllocation, fnc *ValidationConfig) (string, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("validate w/o prefix", "cr", alloc.GetName(), "prefix", alloc.GetPrefix())

	if msg := fnc.ValidateInputFn(alloc); msg != "" {
		return msg, nil
	}

	return "", nil
}

func ValidateInputNopFn(alloc *ipamv1alpha1.IPAllocation) string { return "" }

func ValidateInputGenericWithPrefixFn(alloc *ipamv1alpha1.IPAllocation) string {
	// we expect some label metadata in the spec for more specific selection later on
	if len(alloc.GetSpecLabels()) == 0 {
		return "prefix cannot have empty labels, otherwise specific selection is not possible"
	}
	return ""
}

func ValidateInputGenericWithoutPrefixFn(alloc *ipamv1alpha1.IPAllocation) string {
	// we expect some label metadata in the spec for prefixes that
	// are either statically provisioned (we dont expect /32 or /128 in a network prefix)
	// for dynamically allocated prefixes we expecte labels, exception is interface specific allocations(which have prefixlength undefined)
	if alloc.GetPrefixLengthFromSpec().Int() != 0 && len(alloc.GetSpecLabels()) == 0 {
		return "prefix cannot have empty labels, otherwise specific selection is not possible"
	}
	return ""
}

func IsAddressNopFn(alloc *ipamv1alpha1.IPAllocation) string { return "" }

func IsAddressGenericFn(alloc *ipamv1alpha1.IPAllocation) string {
	pi := iputil.New(alloc.GetPrefix())
	if pi.IsAddressPrefix() {
		return fmt.Sprintf("a %s cannot be created with address (/32, /128) based prefixes, got %s", alloc.GetPrefixKind(), alloc.GetPrefix())
	}
	return ""
}

func IsAddressInNetNopFn(alloc *ipamv1alpha1.IPAllocation) string { return "" }

func IsAddressInNetGenericFn(alloc *ipamv1alpha1.IPAllocation) string {
	pi := iputil.New(alloc.GetPrefix())
	if pi.GetIPSubnet().String() != pi.GetIPPrefix().String() {
		return fmt.Sprintf("a %s prefix cannot have net <> address", alloc.GetPrefixKind())
	}
	return ""
}

func ExactMatchPrefixNetworkFn(alloc *ipamv1alpha1.IPAllocation) netip.Prefix {
	return iputil.New(alloc.GetPrefix()).GetIPSubnet()
}

func ExactPrefixMatchNetworkFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	// validate the owner key
	if route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey) != alloc.GetOwnerGvk() {
		return fmt.Sprintf("route was already allocated from a different origin, new origin %s, ipam origin %s",
			alloc.GetOwnerGvk(),
			route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey))
	}
	if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindNetwork) {
		return fmt.Sprintf("%s prefix in use by %s",
			alloc.GetPrefixKind(),
			route.Labels().Get(ipamv1alpha1.NephioNsnKey))
	}
	for k, specValue := range alloc.GetSpecLabels() {
		if !route.Labels().Has(k) {
			// key in spec does not exist in the ipam route table
			// newly added in spec -> we allow this
			continue
		}
		// value exists in spec
		if route.Labels().Get(k) != specValue {
			// value from the spec does not match the one in the ipam route table
			// change happened
			return fmt.Sprintf("%s prefix in use by %s",
				alloc.GetPrefixKind(),
				route.Labels().String())
		}
		// what to do if a label entry is deleted from the spec -> we allow this
	}
	// all good, net exists already
	return ""
}

func ExactPrefixMatchGenericFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	if route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey) != alloc.GetOwnerGvk() {
		return fmt.Sprintf("route was already allocated from a different origin, new origin %s, ipam origin %s",
			alloc.GetOwnerGvk(),
			route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey))
	}
	if route.Labels().Get(ipamv1alpha1.NephioNsnKey) != alloc.GetName() {
		return fmt.Sprintf("%s prefix in use by %s",
			alloc.GetPrefixKind(),
			route.Labels().Get(ipamv1alpha1.NephioNsnKey))
	}
	// TBD what do we do if there are changes
	// -> change prefixKind
	// -> change labels
	// right now we all the change

	// all good, net exists already
	return ""
}

func ChildrenExistNopFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string { return "" }

func ChildrenExistLoopbackFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	return fmt.Sprintf("a more specific prefix was already allocated %s, loopbacks need to be created in hierarchy",
		route.Labels().Get(ipamv1alpha1.NephioNsnKey))
}

func ChildrenExistGenericFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	return fmt.Sprintf("a more specific prefix was already allocated %s, nesting not allowed for %s",
		route.Labels().Get(ipamv1alpha1.NephioNsnKey),
		alloc.GetPrefixKind(),
	)
}

func NoParentExistGenericFn(alloc *ipamv1alpha1.IPAllocation) string {
	return fmt.Sprintf("an aggregate prefix is required for: %s", alloc.GetPrefix())
}

func NoParentExistAggregateFn(alloc *ipamv1alpha1.IPAllocation) string {
	if alloc.GetOwnerGvk() == ipamv1alpha1.NetworkInstanceGVKString {
		// aggregates from a network instance dont need a parent since they
		// are the parent for the network instance
		return ""
	}
	return fmt.Sprintf("an aggregate prefix is required for: %s", alloc.GetPrefix())
}

func ParentExistNetworkFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	//fmt.Printf("ParentExistNetworkFn route: %s\n", route.String())
	// if the parent is not an aggregate we dont allow the prefix to be created
	if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
		return fmt.Sprintf("nesting network prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s of kind %s",
			route.Labels().Get(ipamv1alpha1.NephioNsnKey),
			route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey),
		)
	}
	return ""
}

func ParentExistLoopbackFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	pi := iputil.New(alloc.GetPrefix())
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) &&
		route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindLoopback) {
		return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/loopback prefix is not allowed, prefix nested with %s",
			route.Labels().Get(ipamv1alpha1.NephioNsnKey))
	}
	if pi.IsAddressPrefix() {
		// address (/32 or /128) can parant with aggregate or loopback
		switch route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) {
		case string(ipamv1alpha1.PrefixKindAggregate), string(ipamv1alpha1.PrefixKindLoopback):
			// /32 or /128 can be parented with aggregates or loopbacks
		default:
			return fmt.Sprintf("nesting loopback prefixes only possible with address (/32, /128) based prefixes, got %s", pi.GetIPPrefix().String())
		}
	}

	if !pi.IsAddressPrefix() {
		switch route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) {
		case string(ipamv1alpha1.PrefixKindAggregate):
			// none /32 or /128 can only be parented with aggregates
		default:
			return fmt.Sprintf("nesting (none /32, /128)loopback prefixes only possible with aggregate prefixes, got %s", route.String())
		}
	}
	return ""
}

func ParentExistPoolFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	//p := alloc.GetIPPrefix()
	// if the parent is not an aggregate we dont allow the prefix to be created
	if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) &&
		route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindPool) {
		return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/pool prefix is not allowed, prefix nested with %s",
			route.Labels().Get(ipamv1alpha1.NephioNsnKey))
	}
	return ""
}

func ParentExistAggregateFn(alloc *ipamv1alpha1.IPAllocation, route table.Route) string {
	// if the parent is not an aggregate we dont allow the prefix to eb create
	if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
		return fmt.Sprintf("nesting aggregate prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s",
			route.Labels().Get(ipamv1alpha1.NephioNsnKey))
	}
	return ""
}

func FinalValidationNopFn(alloc *ipamv1alpha1.IPAllocation, dryrunrt *table.RIB) string {
	return ""
}

func FinalValidationNetworkFn(alloc *ipamv1alpha1.IPAllocation, dryrunrt *table.RIB) string {
	l, err := alloc.GetSubnetLabelSelector()
	if err != nil {
		return err.Error()
	}
	routes := dryrunrt.GetByLabel(l)
	for _, route := range routes {
		fmt.Printf("final validation network: %s, labels: %v\n", route.String(), route.Labels()
	}
	return ""
}
*/
