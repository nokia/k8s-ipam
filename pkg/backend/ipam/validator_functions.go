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
	"fmt"

	"github.com/hansthienpondt/nipam/pkg/table"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
)

func validateInput(claim *ipamv1alpha1.IPClaim, pi *iputil.Prefix) string {
	if pi == nil {
		if claim.Spec.Kind == ipamv1alpha1.PrefixKindAggregate {
			return fmt.Sprintf("a dynamic prefix claim is not supported for: %s", claim.Spec.Kind)
		}
		// this is a claim w/o a prefix
		if claim.Spec.CreatePrefix != nil {
			// this is request for a dynamic prefix claim
			// based on prefixlength set by the user, the prefix length has to be specified
			if claim.Spec.PrefixLength == nil {
				return "a dynamic prefix claimation w/o a prefix length need a prefix length to be set"
			}
			return ""
		}
		if claim.Spec.PrefixLength != nil {
			return fmt.Sprintf("a dynamic prefix claimation with prefixlength <> 0 need to set create prefix, got: %d", claim.Spec.PrefixLength)
		}
		if claim.Spec.Kind == ipamv1alpha1.PrefixKindAggregate ||
			claim.Spec.Kind == ipamv1alpha1.PrefixKindPool {
			return fmt.Sprintf("a dynamic claim of kind %s is not supported unless prefixlength and create prefix is set", claim.Spec.Kind)
		}
		return ""
	}
	// validate generic prefix handling
	// validate address based prefix if prefix has a notation of /32 or /128
	if claim.Spec.Kind == ipamv1alpha1.PrefixKindAggregate ||
		claim.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
		if pi.IsAddressPrefix() {
			return fmt.Sprintf("a prefix claim for kind %s is not allowed with /32 for ipv4 or /128 for ipv6 notation", claim.Spec.Kind)
		}
	}

	// validation check net <> address
	if claim.Spec.Kind != ipamv1alpha1.PrefixKindNetwork {
		if pi.GetIPSubnet().String() != pi.GetIPPrefix().String() {
			return fmt.Sprintf("net <> address is not allowed for prefixkind: %s", claim.Spec.Kind)
		}
	}

	if claim.Spec.CreatePrefix != nil {
		if pi.IsAddressPrefix() {
			// a create prefix should have an address different from /32 or /128
			return fmt.Sprintf("create prefix is not allowed with /32 for ipv4 or /128 for ipv6, got: %s", pi.GetIPPrefix().String())
		}
		return ""
	}
	return ""
}

func validatePrefixOwner(route table.Route, claim *ipamv1alpha1.IPClaim) string {
	if route.Labels()[resourcev1alpha1.NephioNsnNamespaceKey] != claim.GetUserDefinedLabels()[resourcev1alpha1.NephioNsnNamespaceKey] ||
		route.Labels()[resourcev1alpha1.NephioNsnNameKey] != claim.GetUserDefinedLabels()[resourcev1alpha1.NephioNsnNameKey] ||
		route.Labels()[resourcev1alpha1.NephioOwnerNsnNamespaceKey] != claim.GetUserDefinedLabels()[resourcev1alpha1.NephioOwnerNsnNamespaceKey] ||
		route.Labels()[resourcev1alpha1.NephioOwnerNsnNameKey] != claim.GetUserDefinedLabels()[resourcev1alpha1.NephioOwnerNsnNameKey] ||
		route.Labels()[resourcev1alpha1.NephioOwnerGvkKey] != claim.GetUserDefinedLabels()[resourcev1alpha1.NephioOwnerGvkKey] {
		return fmt.Sprintf("%s by owner gvk %s, owner nsn %s/%s with nsn %s/%s",
			errValidateDuplicatePrefix,
			route.Labels()[resourcev1alpha1.NephioOwnerGvkKey],
			route.Labels()[resourcev1alpha1.NephioOwnerNsnNamespaceKey],
			route.Labels()[resourcev1alpha1.NephioOwnerNsnNameKey],
			route.Labels()[resourcev1alpha1.NephioNsnNamespaceKey],
			route.Labels()[resourcev1alpha1.NephioNsnNameKey])
	}

	return ""
}

func validateChildrenExist(route table.Route, prefixKind ipamv1alpha1.PrefixKind) string {
	switch prefixKind {
	case ipamv1alpha1.PrefixKindNetwork:
		if route.Labels()[resourcev1alpha1.NephioPrefixKindKey] != string(ipamv1alpha1.PrefixKindNetwork) {
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
		return fmt.Sprintf("a more specific prefix was already claimed %s/%s, nesting not allowed for %s",
			route.Labels().Get(resourcev1alpha1.NephioNsnNamespaceKey),
			route.Labels().Get(resourcev1alpha1.NephioNsnNameKey),
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

func validateParentExist(route table.Route, claim *ipamv1alpha1.IPClaim, pi *iputil.Prefix) string {
	switch claim.Spec.Kind {
	case ipamv1alpha1.PrefixKindAggregate:
		if route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
			return fmt.Sprintf("nesting aggregate prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s/%s",
				route.Labels().Get(resourcev1alpha1.NephioNsnNamespaceKey),
				route.Labels().Get(resourcev1alpha1.NephioNsnNameKey))
		}
		return ""
	case ipamv1alpha1.PrefixKindLoopback:
		if route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) &&
			route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindLoopback) {
			return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/loopback prefix is not allowed, prefix nested with %s/%s",
				route.Labels().Get(resourcev1alpha1.NephioNsnNamespaceKey),
				route.Labels().Get(resourcev1alpha1.NephioNsnNameKey))
		}
		if pi.IsAddressPrefix() {
			// address (/32 or /128) can parant with aggregate or loopback
			switch route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) {
			case string(ipamv1alpha1.PrefixKindAggregate), string(ipamv1alpha1.PrefixKindLoopback):
				// /32 or /128 can be parented with aggregates or loopbacks
			default:
				return fmt.Sprintf("nesting loopback prefixes only possible with address (/32, /128) based prefixes, got %s", pi.GetIPPrefix().String())
			}
		}

		if !pi.IsAddressPrefix() {
			switch route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) {
			case string(ipamv1alpha1.PrefixKindAggregate):
				// none /32 or /128 can only be parented with aggregates
			default:
				return fmt.Sprintf("nesting (none /32, /128)loopback prefixes only possible with aggregate prefixes, got %s", route.String())
			}
		}
		return ""
	case ipamv1alpha1.PrefixKindNetwork:
		if claim.Spec.CreatePrefix != nil {
			if route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) {
				return fmt.Sprintf("nesting network prefixes with anything other than an aggregate prefix is not allowed, prefix nested with %s/%s of kind %s",
					route.Labels().Get(resourcev1alpha1.NephioNsnNamespaceKey),
					route.Labels().Get(resourcev1alpha1.NephioNsnNameKey),
					route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey),
				)
			}
		} else {
			if route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindNetwork) {
				return fmt.Sprintf("%s, got %s/%s of kind %s",
					errValidateNetworkPrefixWoNetworkParent,
					route.Labels().Get(resourcev1alpha1.NephioNsnNamespaceKey),
					route.Labels().Get(resourcev1alpha1.NephioNsnNameKey),
					route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey),
				)
			}
		}

		return ""
	case ipamv1alpha1.PrefixKindPool:
		// if the parent is not an aggregate we dont allow the prefix to be created
		if route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindAggregate) &&
			route.Labels().Get(resourcev1alpha1.NephioPrefixKindKey) != string(ipamv1alpha1.PrefixKindPool) {
			return fmt.Sprintf("nesting loopback prefixes with anything other than an aggregate/pool prefix is not allowed, prefix nested with %s/%s",
				route.Labels().Get(resourcev1alpha1.NephioNsnNamespaceKey),
				route.Labels().Get(resourcev1alpha1.NephioNsnNameKey))
		}
		return ""
	}
	return ""
}
