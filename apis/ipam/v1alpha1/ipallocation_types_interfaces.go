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

package v1alpha1

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hansthienpondt/goipam/pkg/table"
	"github.com/nokia/k8s-ipam/internal/meta"
	"inet.af/netaddr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCondition of this resource
func (r *IPAllocation) GetCondition(ck ConditionKind) Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions of the Network Node.
func (r *IPAllocation) SetConditions(c ...Condition) {
	r.Status.SetConditions(c...)
}

func (r *IPAllocation) GetNetworkInstance() string {
	return r.Spec.NetworkInstance
}

func (r *IPAllocation) GetPrefixKind() PrefixKind {
	return r.Spec.PrefixKind
}

func (r *IPAllocation) GetPrefix() string {
	return r.Spec.Prefix
}

func (r *IPAllocation) GetIPPrefix() netaddr.IPPrefix {
	return netaddr.MustParseIPPrefix(r.Spec.Prefix)
}

func (r *IPAllocation) GetAddressFamily() AddressFamily {
	switch r.Spec.AddressFamily {
	case "ipv4":
		return AddressFamilyIpv4
	case "ipv6":
		return AddressFamilyIpv6
	default:
		return AddressFamilyUnknown
	}
}

func (x *IPAllocation) GetPrefixLength() uint8 {
	return x.Spec.PrefixLength
}

func (x *IPAllocation) GetPrefixLengthFromRoute(route *table.Route) uint8 {
	if x.GetPrefixLength() != 0 {
		return x.Spec.PrefixLength
	}
	if route.IPPrefix().IP().Is4() {
		return 32
	}
	return 128
}

func (x *IPAllocation) GetOwnerGvk() string {
	if len(x.Spec.Labels) != 0 {
		return x.Spec.Labels[NephioOwnerGvkKey]
	}
	return ""
}

func (r *IPAllocation) GetSelectorLabels() map[string]string {
	l := map[string]string{}
	if r.Spec.Selector != nil {
		for k, v := range r.Spec.Selector.MatchLabels {
			l[k] = v
		}
	}
	return l
}

func (r *IPAllocation) GetSpecLabels() map[string]string {
	l := map[string]string{}
	for k, v := range r.Spec.Labels {
		l[k] = v
	}
	return l
}

func (x *IPAllocation) GetSubnetLabelSelector() (labels.Selector, error) {
	//p := netaddr.MustParseIPPrefix(r.Prefix)
	//af := iputil.GetAddressFamily(p)

	l := map[string]string{
		NephioSubnetKey: GetSubnetName(x.GetPrefix()),
		//ipamv1alpha1.NephioSubnetNameKey:    r.SubnetName,
		//ipamv1alpha1.NephioAddressFamilyKey: string(af),
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *IPAllocation) GetGatewayLabelSelector() (labels.Selector, error) {
	l := map[string]string{
		NephioGatewayKey: "true",
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		// exclude any key that is not network and networkinstance
		if k == NephioSubnetKey ||
			//k == ipamv1alpha1.NephioNetworkInstanceKey ||
			k == NephioGatewayKey {
			req, err := labels.NewRequirement(k, selection.In, []string{v})
			if err != nil {
				return nil, err
			}
			fullselector = fullselector.Add(*req)
		}
	}
	for k, v := range r.GetSelectorLabels() {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *IPAllocation) GetLabelSelector() (labels.Selector, error) {
	l := r.GetSelectorLabels()
	// For prefixkind network we want to allocate only prefixes within a network
	if r.GetPrefixKind() == PrefixKindNetwork {
		l[NephioPrefixKindKey] = string(PrefixKindNetwork)
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *IPAllocation) GetAllocSelector() (labels.Selector, error) {
	l := map[string]string{
		NephioIPAllocactionNameKey: r.GetName(),
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *IPAllocation) GetFullLabels() map[string]string {
	l := make(map[string]string)
	for k, v := range r.GetSpecLabels() {
		l[k] = v
	}
	for k, v := range r.GetSelectorLabels() {
		l[k] = v
	}
	return l
}

func (r *IPAllocation) GetPrefixFromNewAlloc() string {
	p := r.GetPrefix()
	parentPrefixLength, ok := r.GetSpecLabels()[NephioParentPrefixLengthKey]
	if ok {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return p
}

// GetAddressFamily returns an address family
func GetAddressFamily(p netaddr.IPPrefix) AddressFamily {
	if p.IP().Is6() {
		return AddressFamilyIpv6
	}
	if p.IP().Is4() {
		return AddressFamilyIpv4
	}
	return AddressFamilyUnknown
}

func IsAddress(p netaddr.IPPrefix) bool {
	af := GetAddressFamily(p)
	prefixLength := GetPrefixLength(p)
	return (af == AddressFamilyIpv4 && (prefixLength == "32")) ||
		(af == AddressFamilyIpv6) && (prefixLength == "128")
}

func GetPrefixLengthFromAlloc(route *table.Route, alloc IPAllocation) uint8 {
	if alloc.Spec.PrefixLength != 0 {
		return alloc.Spec.PrefixLength
	}
	if route.IPPrefix().IP().Is4() {
		return 32
	}
	return 128
}

func GetPrefixFromAlloc(p string, alloc *IPAllocation) string {
	parentPrefixLength, ok := alloc.GetSpecLabels()[NephioParentPrefixLengthKey]
	if ok {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return p
}

func GetPrefixFromRoute(route *table.Route) *string {
	parentPrefixLength := route.GetLabels().Get(NephioParentPrefixLengthKey)
	p := route.IPPrefix().String()
	if parentPrefixLength != "" {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return &p
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

func GetSubnetName(prefix string) string {
	p := netaddr.MustParseIPPrefix(prefix)

	return fmt.Sprintf("%s-%s",
		p.Masked().IP().String(),
		GetPrefixLength(p))
}

func BuildIPAllocationFromIPPrefix(cr *IPPrefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	p := netaddr.MustParseIPPrefix(cr.Spec.Prefix)
	spec := IPAllocationSpec{
		PrefixKind:      cr.Spec.PrefixKind,
		NetworkInstance: cr.Spec.NetworkInstance,
		AddressFamily:   GetAddressFamily(p),
		Prefix:          cr.Spec.Prefix,
		PrefixLength:    uint8(GetPrefixLengthAsInt(p)),
		Labels: AddSpecLabelsWithTypeMeta(
			ownerGvk,
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}.String(),
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}.String(),
			cr.Spec.Labels), // added the owner label in it
	}
	return BuildIPAllocation(cr, cr.GetName(), spec)
}

func BuildIPAllocationFromNetworkInstancePrefix(cr *NetworkInstance, prefix *Prefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	p := netaddr.MustParseIPPrefix(prefix.Prefix)
	spec := IPAllocationSpec{
		PrefixKind:      PrefixKindAggregate,
		NetworkInstance: cr.GetName(),
		AddressFamily:   GetAddressFamily(p),
		Prefix:          prefix.Prefix,
		PrefixLength:    uint8(GetPrefixLengthAsInt(p)),
		Labels: AddSpecLabelsWithTypeMeta(
			ownerGvk,
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}.String(),
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}.String(),
			prefix.Labels), // added the owner label in it
	}
	// name is based on aggregate and prefix
	return BuildIPAllocation(cr, cr.GetNameFromNetworkInstancePrefix(prefix.Prefix), spec)
}

func AddSpecLabelsWithTypeMeta(ownerGvk *schema.GroupVersionKind, ownerNsn, nsn string, specLabels map[string]string) map[string]string {
	labels := map[string]string{
		NephioOwnerGvkKey: meta.GVKToString(ownerGvk),
		NephioOwnerNsnKey: ownerNsn,
		NephioGvkKey:      IPAllocationKindGVKString,
		NephioNsnKey:      nsn,
	}
	for k, v := range specLabels {
		labels[k] = v
	}
	return labels
}

func BuildIPAllocation(o client.Object, crName string, spec IPAllocationSpec) *IPAllocation {
	return &IPAllocation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: IPAllocationKindAPIVersion,
			Kind:       IPAllocationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.GetNamespace(),
			Name:      crName,
			Labels:    o.GetLabels(),
		},
		Spec: spec,
	}
}
