package ipam

import (
	"fmt"

	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
)

func validateInput(alloc *ipamv1alpha1.IPAllocation, pi iputil.PrefixInfo) string {
	if pi == nil {
		// this is an allocation w/o a prefix
		if alloc.GetCreatePrefix() {
			// this is request for a dynamic prefix allocation
			// based on prefixlength set by the user, the prefix length has to be specified
			if alloc.GetPrefixLengthFromSpec() != 0 {
				return ""
			}
			return "a dynamic prefix allocation w/o a prefix length is not valid"
		}
		return ""
	}
	// validate generic prefix handling
	// validate address based prefix if prefix has a notation of /32 or /128
	if alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindAggregate ||
		alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		if pi.IsAddressPrefix() {
			return fmt.Sprintf("no /32 or /128 notation is allowed for prefixkind: %s", alloc.GetPrefixKind())
		}
	}

	if alloc.GetPrefixKind() != ipamv1alpha1.PrefixKindNetwork {
		if pi.GetIPSubnet().String() != pi.GetIPPrefix().String() {
			return fmt.Sprintf("net <> address is not allowed for prefixkind: %s", alloc.GetPrefixKind())
		}
	}

	if alloc.GetCreatePrefix() {
		if pi.IsAddressPrefix() {
			return fmt.Sprintf("create prefix is not allowed with /32 or /128, got: %s", pi.GetIPPrefix().String())
		}
		return ""
	}
	return ""
}

/*
func validateExactMatchLenRoutes(routes table.Routes) string {
	if len(routes) > 1 {
		return "multiple prefixes match the nsn labelselector"
	}
	return ""
}
*/

/*
func validateExactMatchPrefix(route table.Route, pi iputil.PrefixInfo) string {
	if route.Prefix().String() != pi.GetIPAddressPrefix().String() {
		return "route exist but prefix mismatch"
	}
	return ""
}
*/

func validatePrefixOwner(route table.Route, alloc *ipamv1alpha1.IPAllocation) string {
	if route.Labels()[ipamv1alpha1.NephioNsnNamespaceKey] != alloc.GetSpecLabels()[ipamv1alpha1.NephioNsnNamespaceKey] ||
		route.Labels()[ipamv1alpha1.NephioNsnNameKey] != alloc.GetSpecLabels()[ipamv1alpha1.NephioNsnNameKey] {
		return fmt.Sprintf("prefix was already allocated by %s/%s",
			alloc.GetSpecLabels()[ipamv1alpha1.NephioNsnNamespaceKey],
			alloc.GetSpecLabels()[ipamv1alpha1.NephioNsnNameKey])
	}
	return ""
}

/*
func validateExactMatchOwnerGVk(route table.Route, alloc *ipamv1alpha1.IPAllocation) string {
	if route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey) != alloc.GetOwnerGvk() {
		return fmt.Sprintf("route was already allocated from a different origin, req origin %s, ipam origin %s",
			alloc.GetOwnerGvk(),
			route.Labels().Get(ipamv1alpha1.NephioOwnerGvkKey))
	}
	return ""
}
*/

/*
func validateExactMatcPrefixKind(route table.Route, alloc *ipamv1alpha1.IPAllocation) string {
	if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(alloc.GetPrefixKind()) {
		return fmt.Sprintf("prefix used by different prefixKind req prefixKind %s, ipam prefixKind %s",
			string(alloc.GetPrefixKind()),
			route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey))
	}
	return ""
}
*/

func validateChildrenExist(route table.Route, prefixKind ipamv1alpha1.PrefixKind) string {
	switch prefixKind {
	case ipamv1alpha1.PrefixKindNetwork:
		if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] != string(ipamv1alpha1.PrefixKindNetwork) {
			return fmt.Sprintf("a child prefix of a %s prefix, can be of the same kind", prefixKind)
		}
		if route.Prefix().Addr().Is4() && route.Prefix().Bits() != 32 {
			return fmt.Sprintf("a child prefix of a %s prefix, can only be an address prefix (/32), got: %v", prefixKind, route.Prefix())
		}
		if route.Prefix().Addr().Is6() && route.Prefix().Bits() != 128 {
			return fmt.Sprintf("a child prefix of a %s prefix, can only be an address prefix (/128), got: %v", prefixKind, route.Prefix())
		}
		return ""
	case ipamv1alpha1.PrefixKindAggregate:
		// nesting is possible in aggregate
		return ""
	default:
		return fmt.Sprintf("a more specific prefix was already allocated %s/%s, nesting not allowed for %s",
			route.Labels().Get(ipamv1alpha1.NephioNsnNamespaceKey),
			route.Labels().Get(ipamv1alpha1.NephioNsnNameKey),
			prefixKind)
	}
}

func validateNoParentExist(prefixKind ipamv1alpha1.PrefixKind, ownerGvk string) string {
	if ownerGvk == ipamv1alpha1.NetworkInstanceKindGVKString {
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
			return fmt.Sprintf("nesting aggregate prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s/%s",
				route.Labels().Get(ipamv1alpha1.NephioNsnNamespaceKey),
				route.Labels().Get(ipamv1alpha1.NephioNsnNameKey))
		}
		return ""
	case ipamv1alpha1.PrefixKindLoopback:
		if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) &&
			route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindLoopback) {
			return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/loopback prefix is not allowed, prefix nested with %s/%s",
				route.Labels().Get(ipamv1alpha1.NephioNsnNamespaceKey),
				route.Labels().Get(ipamv1alpha1.NephioNsnNameKey))
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
			return fmt.Sprintf("nesting network prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s/%s of kind %s",
				route.Labels().Get(ipamv1alpha1.NephioNsnNamespaceKey),
				route.Labels().Get(ipamv1alpha1.NephioNsnNameKey),
				route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey),
			)
		}
		return ""
	case ipamv1alpha1.PrefixKindPool:
		// if the parent is not an aggregate we dont allow the prefix to be created
		if route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) &&
			route.Labels().Get(ipamv1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindPool) {
			return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/pool prefix is not allowed, prefix nested with %s/%s",
				route.Labels().Get(ipamv1alpha1.NephioNsnNamespaceKey),
				route.Labels().Get(ipamv1alpha1.NephioNsnNameKey))
		}
		return ""
	}
	return ""
}
