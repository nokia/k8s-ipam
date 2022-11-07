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

package iputil

import (
	"strconv"
	"strings"

	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"inet.af/netaddr"
)

// GetAddressFamily returns an address family
func GetAddressFamily(p netaddr.IPPrefix) ipamv1alpha1.AddressFamily {
	if p.IP().Is6() {
		return ipamv1alpha1.AddressFamilyIpv6
	}
	if p.IP().Is4() {
		return ipamv1alpha1.AddressFamilyIpv4
	}
	return ipamv1alpha1.AddressFamilyUnknown
}

// GetPrefixLength returns a prefix length in string format
func GetPrefixLength(p netaddr.IPPrefix) string {
	prefixSize, _ := p.IPNet().Mask.Size()
	return strconv.Itoa(prefixSize)
}

// GetPrefixLength returns a prefix length in int format
func GetPrefixLengthAsInt(p netaddr.IPPrefix) int {
	prefixSize, _ := p.IPNet().Mask.Size()
	return prefixSize
}

// GetAddressPrefixLength return the prefix lenght of the address in the prefix
// used only for IP addresses
func GetAddressPrefixLength(p netaddr.IPPrefix) string {
	if p.IP().Is6() {
		return "128"
	}
	return "32"
}

// GetAddress return a string prefix notation for an address
func GetAddress(p netaddr.IPPrefix) string {
	addressPrefixLength := GetAddressPrefixLength(p)
	return strings.Join([]string{p.IP().String(), addressPrefixLength}, "/")
}

// GetAddress return a string prefix notation for an address
func GetFirstAddress(p netaddr.IPPrefix) string {
	addressPrefixLength := GetAddressPrefixLength(p)
	return strings.Join([]string{p.Masked().IP().String(), addressPrefixLength}, "/")
}

func IsAddress(p netaddr.IPPrefix) bool {
	af := GetAddressFamily(p)
	prefixLength := GetPrefixLength(p)
	return (af == ipamv1alpha1.AddressFamilyIpv4 && (prefixLength == "32")) ||
		(af == ipamv1alpha1.AddressFamilyIpv6) && (prefixLength == "128")
}

func GetPrefixLengthFromAlloc(route *table.Route, alloc *ipamv1alpha1.IPAllocation) uint8 {
	if alloc.Spec.PrefixLength != 0 {
		return alloc.Spec.PrefixLength
	}
	if route.IPPrefix().IP().Is4() {
		return 32
	}
	return 128
}

func GetPrefixFromAlloc(p string, alloc *ipamv1alpha1.IPAllocation) string {
	parentPrefixLength, ok := alloc.GetLabels()[ipamv1alpha1.NephioParentPrefixLengthKey]
	if ok {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return p
}

func GetPrefixFromRoute(route *table.Route) *string {
	parentPrefixLength := route.GetLabels().Get(ipamv1alpha1.NephioParentPrefixLengthKey)
	p := route.IPPrefix().String()
	if parentPrefixLength != "" {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return &p
}
