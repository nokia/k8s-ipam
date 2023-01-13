package ipam

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Validator interface {
	Validate(ctx context.Context) (string, error)
}

type validateExistanceOfSpecialLabelsFn func(labels map[string]string) string
type validateAddressPrefixFn func(pi iputil.PrefixInfo) string
type validateIfAddressinSubnetFn func(pi iputil.PrefixInfo, prefixKind ipamv1alpha1.PrefixKind) string
type validateChildrenExistFn func(route table.Route, prefixKind ipamv1alpha1.PrefixKind) string
type validateNoParentExistFn func(prefixKind ipamv1alpha1.PrefixKind, ownerGvk string) string
type validateParentExistFn func(route table.Route, prefixKind ipamv1alpha1.PrefixKind, pi iputil.PrefixInfo) string

// func validateExistanceOfSpecialLabelsNop(labels map[string]string) string { return "" }
func validateAddressPrefixNop(pi iputil.PrefixInfo) string { return "" }
func validateIfAddressinSubnetNop(pi iputil.PrefixInfo, prefixKind ipamv1alpha1.PrefixKind) string {
	return ""
}

//func validateChildrenExistNop(route table.Route, prefixKind ipamv1alpha1.PrefixKind) string {
//	return ""
//}
//func validateNoParentExistNop(prefixKind ipamv1alpha1.PrefixKind, ownerGvk string) string { return "" }
//func validateParentExistNop(route table.Route, prefixKind ipamv1alpha1.PrefixKind, pi iputil.PrefixInfo) string {
//	return ""
//}

type PrefixValidatorFunctionConfig struct {
	validateExistanceOfSpecialLabelsFn validateExistanceOfSpecialLabelsFn
	validateAddressPrefixFn            validateAddressPrefixFn
	validateIfAddressinSubnetFn        validateIfAddressinSubnetFn
	validateChildrenExistFn            validateChildrenExistFn
	validateNoParentExistFn            validateNoParentExistFn
	validateParentExistFn              validateParentExistFn
}

type PrefixValidatorConfig struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	pi    iputil.PrefixInfo
	fnc   *PrefixValidatorFunctionConfig
}

func NewPrefixValidator(c *PrefixValidatorConfig) Validator {
	return &prefixvalidator{
		alloc: c.alloc,
		rib:   c.rib,
		pi:    c.pi,
		fnc:   c.fnc,
	}
}

type prefixvalidator struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	pi    iputil.PrefixInfo
	fnc   *PrefixValidatorFunctionConfig
	l     logr.Logger
}

func (r *prefixvalidator) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("prefixkind", r.alloc.GetPrefixKind(), "cr", r.alloc.GetName(), "prefix", r.alloc.GetPrefix())
	r.l.Info("validate")

	// get dryrun rib
	dryrunRib := r.rib.Clone()

	// validate input
	r.l.Info("validate", "fnc", r.fnc)
	if msg := r.fnc.validateExistanceOfSpecialLabelsFn(r.alloc.GetSpecLabels()); msg != "" {
		return msg, nil
	}
	// validate address
	if msg := r.fnc.validateAddressPrefixFn(r.pi); msg != "" {
		return msg, nil
	}
	// validate address in subnet
	if msg := r.fnc.validateAddressPrefixFn(r.pi); msg != "" {
		return msg, nil
	}

	// exact match prefix
	allocSelector, err := r.alloc.GetAllocSelector()
	if err != nil {
		return "", err
	}
	routes := dryrunRib.GetByLabel(allocSelector)
	// Route exists
	if len(routes) > 0 {
		// check if more than 1 route exist which would be bad
		if msg := validateExactMatchLenRoutes(routes); msg != "" {
			return msg, nil
		}
		// validate the prefix
		if msg := validateExactMatchPrefix(routes[0], r.pi); msg != "" {
			return msg, nil
		}
		// validate the owner gvk match
		if msg := validateExactMatchOwnerGVk(routes[0], r.alloc); msg != "" {
			return msg, nil
		}
		// validate the prefixKind
		if msg := validateExactMatcPrefixKind(routes[0], r.alloc); msg != "" {
			return msg, nil
		}
	}
	// Route does not exist
	// add dummy route in dry run rib, this is the subnet route
	// which serves as general purpose
	route := table.NewRoute(
		r.pi.GetIPSubnet(),
		r.alloc.GetDummyLabelsFromPrefix(r.pi),
		map[string]any{},
	)
	// if the prefix already exists we need to set it otherwise add it
	if _, ok := dryrunRib.Get(route.Prefix()); ok {
		if err := dryrunRib.Set(route); err != nil {
			r.l.Error(err, "cannot set route", "route", route)
			return "", err
		}
	} else {
		if err := dryrunRib.Add(route); err != nil {
			r.l.Error(err, "cannot add route", "route", route)
			return "", err
		}
	}

	// get the route again and check for children
	route, ok := dryrunRib.Get(r.pi.GetIPSubnet())
	if !ok {
		return "", fmt.Errorf("route just added, but a new get does not find it")
	}
	// check for children
	routes = route.Children(dryrunRib)
	if len(routes) > 0 {
		r.l.Info("got children", "routes", routes)
		if msg := r.fnc.validateChildrenExistFn(routes[0], r.alloc.GetPrefixKind()); msg != "" {
			return msg, nil
		}
	}
	// get parents
	routes = route.Parents(dryrunRib)
	if len(routes) == 0 {
		if msg := r.fnc.validateNoParentExistFn(r.alloc.GetPrefixKind(), r.alloc.GetOwnerGvk()); msg != "" {
			return msg, nil
		}
	}
	for _, route := range routes {
		if msg := r.fnc.validateParentExistFn(route, r.alloc.GetPrefixKind(), r.pi); msg != "" {
			return msg, nil
		}
	}
	// final validation
	//if msg := validateFinal(); msg != "" {
	//	return msg, nil
	//}
	return "", nil
}

func validateExistanceOfSpecialLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "prefix cannot have empty labels, otherwise specific selection is not possible"
	}
	return ""
}

func validateAddressPrefix(pi iputil.PrefixInfo) string {
	if pi.IsAddressPrefix() {
		return "the prefix cannot be created with address (/32, /128)"
	}
	return ""
}

func validateIfAddressinSubnet(pi iputil.PrefixInfo, prefixKind ipamv1alpha1.PrefixKind) string {
	if pi.GetIPSubnet().String() != pi.GetIPPrefix().String() {
		return fmt.Sprintf("a %s prefix cannot have net <> address", prefixKind)
	}
	return ""
}

func validateExactMatchLenRoutes(routes table.Routes) string {
	if len(routes) > 1 {
		return "multiple prefixes match the nsn labelselector"
	}
	return ""
}

func validateExactMatchPrefix(route table.Route, pi iputil.PrefixInfo) string {
	if route.Prefix().String() != pi.GetIPAddressPrefix().String() {
		return "route exist but prefix mismatch"
	}
	return ""
}

func validateExactMatchOwnerGVk(route table.Route, alloc *ipamv1alpha1.IPAllocation) string {
	if route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey) != alloc.GetOwnerGvk() {
		return fmt.Sprintf("route was already allocated from a different origin, req origin %s, ipam origin %s",
			alloc.GetOwnerGvk(),
			route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey))
	}
	return ""
}

func validateExactMatcPrefixKind(route table.Route, alloc *ipamv1alpha1.IPAllocation) string {
	if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(alloc.GetPrefixKind()) {
		return fmt.Sprintf("prefix used by different prefixKind req prefixKind %s, ipam prefixKind %s",
			string(alloc.GetPrefixKind()),
			route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey))
	}
	return ""
}

func validateChildrenExist(route table.Route, prefixKind ipamv1alpha1.PrefixKind) string {
	if prefixKind == ipamv1alpha1.PrefixKindNetwork {
		if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] != string(ipamv1alpha1.PrefixKindNetwork) {
			return fmt.Sprintf("a child prefix of a %s prefix, can be of the same kind", prefixKind)
		}
		if route.Prefix().Addr().Is4() && route.Prefix().Bits() != 32 {
			return fmt.Sprintf("a child prefix of a %s prefix, can only be an address prefix (/32), got: %v", prefixKind, route.Prefix())
		}
		if route.Prefix().Addr().Is6() && route.Prefix().Bits() != 128 {
			return fmt.Sprintf("a child prefix of a %s prefix, can only be an address prefix (/128), got: %v", prefixKind, route.Prefix())
		}
	} else {
		return fmt.Sprintf("a more specific prefix was already allocated %s, nesting not allowed for %s",
			route.Labels().Get(ipamv1alpha1.NephioNsnKey),
			prefixKind)
	}
	return ""
}

func validateNoParentExist(prefixKind ipamv1alpha1.PrefixKind, ownerGvk string) string {
	if prefixKind != ipamv1alpha1.PrefixKindAggregate {
		return "an aggregate prefix is required"
	}
	if ownerGvk == ipamv1alpha1.NetworkInstanceGVKString {
		// aggregates from a network instance dont need a parent since they
		// are the parent for the network instance
		return ""
	}
	return "an aggregate prefix is required"
}

func validateParentExist(route table.Route, prefixKind ipamv1alpha1.PrefixKind, pi iputil.PrefixInfo) string {
	switch prefixKind {
	case ipamv1alpha1.PrefixKindAggregate:
		if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
			return fmt.Sprintf("nesting aggregate prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s",
				route.Labels().Get(ipamv1alpha1.NephioNsnKey))
		}
		return ""
	case ipamv1alpha1.PrefixKindLoopback:
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
	case ipamv1alpha1.PrefixKindNetwork:
		if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
			return fmt.Sprintf("nesting network prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s of kind %s",
				route.Labels().Get(ipamv1alpha1.NephioNsnKey),
				route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey),
			)
		}
		return ""
	case ipamv1alpha1.PrefixKindPool:
		// if the parent is not an aggregate we dont allow the prefix to be created
		if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) &&
			route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindPool) {
			return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/pool prefix is not allowed, prefix nested with %s",
				route.Labels().Get(ipamv1alpha1.NephioNsnKey))
		}
		return ""
	}
	return ""
}
