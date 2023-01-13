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

	//"github.com/hansthienpondt/goipam/pkg/table"
	"github.com/hansthienpondt/nipam/pkg/table"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
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

func (r *IPAllocation) GetAddressFamily() iputil.AddressFamily {
	switch r.Spec.AddressFamily {
	case "ipv4":
		return iputil.AddressFamilyIpv4
	case "ipv6":
		return iputil.AddressFamilyIpv6
	default:
		return iputil.AddressFamilyUnknown
	}
}

func (x *IPAllocation) GetPrefixLengthFromSpec() iputil.PrefixLength {
	return iputil.PrefixLength(x.Spec.PrefixLength)
}

func (x *IPAllocation) GetPrefixLengthFromRoute(route table.Route) iputil.PrefixLength {
	if x.GetPrefixLengthFromSpec() != 0 {
		return x.GetPrefixLengthFromSpec()
	}
	return iputil.PrefixLength(route.Prefix().Addr().BitLen())
}

func (x *IPAllocation) GetGenericNamespacedName() string {
	return fmt.Sprintf("%s-%s", x.GetNamespace(), x.GetName())
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
	pi, err := iputil.New(x.GetPrefix())
	if err != nil {
		return nil, err
	}
	l := map[string]string{
		NephioSubnetKey: pi.GetSubnetName(),
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
		NephioNsnKey: r.Spec.Labels[NephioNsnKey],
	}
	/*
		if r.Spec.Labels[NephioGvkKey] == OriginSystem {
			l[NephioNsnKey] = r.Spec.Labels[NephioNsnKey]
		} else {
			l[NephioNsnKey] = strings.ReplaceAll(
				types.NamespacedName{Name: r.GetName(), Namespace: r.GetNamespace()}.String(), "/", "-",
			)
		}
	*/

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

func GetPrefixLengthFromAlloc(route table.Route, alloc IPAllocation) uint8 {
	if alloc.Spec.PrefixLength != 0 {
		return alloc.Spec.PrefixLength
	}
	if route.Prefix().Addr().Is4() {
		return 32
	}
	return 128
}

func GetPrefixFromAlloc(p string, alloc *IPAllocation) string {
	parentPrefixLength, ok := alloc.GetSpecLabels()[NephioParentPrefixLengthKey]
	if ok {
		pl, _ := strconv.Atoi(parentPrefixLength)
		pi, err := iputil.New(p)
		if err != nil {
			return ""
		}
		p = pi.GetIPPrefixWithPrefixLength(pl).String()
	}
	return p
}

func GetPrefixFromRoute(route table.Route) string {
	parentPrefixLength := route.Labels().Get(NephioParentPrefixLengthKey)
	p := route.Prefix().String()
	if parentPrefixLength != "" {
		pl, _ := strconv.Atoi(parentPrefixLength)
		pi, err := iputil.New(p)
		if err != nil {
			return ""
		}
		p = pi.GetIPPrefixWithPrefixLength(pl).String()
	}
	return p
}

func BuildIPAllocationFromIPPrefix(cr *IPPrefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	pi, err := iputil.New(cr.Spec.Prefix)
	if err != nil {
		return nil
	}
	spec := IPAllocationSpec{
		PrefixKind:      cr.Spec.PrefixKind,
		NetworkInstance: cr.Spec.NetworkInstance,
		AddressFamily:   pi.GetAddressFamily(),
		Prefix:          cr.Spec.Prefix,
		PrefixLength:    uint8(pi.GetPrefixLength().Int()),
		Labels: AddSpecLabelsWithTypeMeta(
			ownerGvk,
			cr.GetGenericNamespacedName(),
			cr.GetGenericNamespacedName(),
			cr.Spec.Labels), // added the owner label in it
	}
	return BuildIPAllocation(cr, cr.GetName(), spec)
}

func BuildIPAllocationFromNetworkInstancePrefix(cr *NetworkInstance, prefix *Prefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	pi, err := iputil.New(prefix.Prefix)
	if err != nil {
		return nil
	}
	spec := IPAllocationSpec{
		PrefixKind:      PrefixKindAggregate,
		NetworkInstance: cr.GetName(),
		AddressFamily:   pi.GetAddressFamily(),
		Prefix:          prefix.Prefix,
		PrefixLength:    uint8(pi.GetPrefixLength().Int()),
		Labels: AddSpecLabelsWithTypeMeta(
			ownerGvk,
			cr.GetGenericNamespacedName(),
			cr.GetGenericNamespacedName(),
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

// GetDummyLabelsFromPrefix used in validation
func (x *IPAllocation) GetDummyLabelsFromPrefix(pi iputil.PrefixInfo) map[string]string {
	return map[string]string{
		NephioOwnerGvkKey:     "dummy",
		NephioOwnerNsnKey:     "dummy",
		NephioGvkKey:          IPAllocationKindGVKString,
		NephioNsnKey:          x.GetGenericNamespacedName(),
		NephioPrefixKindKey:   string(x.GetPrefixKind()),
		NephioPrefixLengthKey: pi.GetPrefixLength().String(),
		NephioSubnetKey:       pi.GetSubnetName(),
	}
}
