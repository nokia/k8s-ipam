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
	"encoding/json"
	"strings"

	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/pkg/errors"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

// +k8s:deepcopy-gen=false
type Allocation struct {
	NamespacedName  types.NamespacedName       `json:"namespacedName,omitempty"`
	Origin          ipamv1alpha1.Origin        `json:"origin,omitempty"`
	NetworkInstance string                     `json:"networkInstance,omitempty"`
	PrefixKind      ipamv1alpha1.PrefixKind    `json:"prefixKind,omitempty"`
	AddresFamily    ipamv1alpha1.AddressFamily `json:"addressFamily,omitempty"` // only used for alloc w/o prefix
	Prefix          string                     `json:"prefix,omitempty"`
	PrefixLength    uint8                      `json:"prefixLength,omitempty"` // only used for alloc w/o prefix and prefix kind = pool
	Network         string                     `json:"network,omitempty"`      // explicitly mentioned for prefixkind network
	Labels          map[string]string          `json:"labels,omitempty"`
	SelectorLabels  map[string]string          `json:"selectorLabels,omitempty"`
	//specificLabels  map[string]string
}

type AllocatedPrefix struct {
	AllocatedPrefix string
	Gateway         string
}

func (r *Allocation) GetName() string {
	return r.NamespacedName.Name
}

func (r *Allocation) GetNameSpace() string {
	return r.NamespacedName.Namespace
}

func (r *Allocation) GetOrigin() ipamv1alpha1.Origin {
	return r.Origin
}

func (r *Allocation) GetNetworkInstance() string {
	return r.NetworkInstance
}

func (r *Allocation) GetNINamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: r.NamespacedName.Namespace,
		Name:      r.NetworkInstance,
	}
}

func (r *Allocation) GetPrefixKind() ipamv1alpha1.PrefixKind {
	return r.PrefixKind
}

func (r *Allocation) GetAddressFamily() ipamv1alpha1.AddressFamily {
	return r.AddresFamily
}

func (r *Allocation) GetPrefix() string {
	return r.Prefix
}

func (r *Allocation) GetIPPrefix() netaddr.IPPrefix {
	return netaddr.MustParseIPPrefix(r.Prefix)
}

func (r *Allocation) GetNetwork() string {
	return r.Network
}

func (r *Allocation) GetLabels() map[string]string {
	l := map[string]string{}
	for k, v := range r.Labels {
		l[k] = v
	}
	return l
}

func (r *Allocation) GetSelectorLabels() map[string]string {
	l := map[string]string{}
	for k, v := range r.SelectorLabels {
		l[k] = v
	}
	return l
}

func (r *Allocation) GetGatewayLabelSelector() (labels.Selector, error) {
	l := map[string]string{
		ipamv1alpha1.NephioGatewayKey: "true",
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		// exclude any key that is not network and networkinstance
		if k == ipamv1alpha1.NephioNetworkNameKey ||
			k == ipamv1alpha1.NephioNetworkInstanceKey ||
			k == ipamv1alpha1.NephioGatewayKey {
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

func (r *Allocation) GetNetworkLabelSelector() (labels.Selector, error) {
	p := netaddr.MustParseIPPrefix(r.Prefix)
	af := iputil.GetAddressFamily(p)

	l := map[string]string{
		ipamv1alpha1.NephioNetworkNameKey:   r.Network,
		ipamv1alpha1.NephioAddressFamilyKey: string(af),
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

func (r *Allocation) GetLabelSelector() (labels.Selector, error) {
	l := r.GetSelectorLabels()
	// For prefixkind network we want to allocate only prefixes within a network
	if r.PrefixKind == ipamv1alpha1.PrefixKindNetwork {
		l[ipamv1alpha1.NephioPrefixKindKey] = string(ipamv1alpha1.PrefixKindNetwork)
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

func (r *Allocation) GetAllocSelector() (labels.Selector, error) {
	l := map[string]string{
		ipamv1alpha1.NephioIPAllocactionNameKey: r.GetName(),
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

func (r *Allocation) GetFullSelector() (labels.Selector, error) {
	fullselector := labels.NewSelector()
	for k, v := range r.GetLabels() {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
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

func (r *Allocation) GetFullLabels() map[string]string {
	l := make(map[string]string)
	for k, v := range r.GetLabels() {
		l[k] = v
	}
	for k, v := range r.GetSelectorLabels() {
		l[k] = v
	}
	return l
}

func (r *Allocation) GetPrefixFromNewAlloc() string {
	p := r.Prefix
	parentPrefixLength, ok := r.GetLabels()[ipamv1alpha1.NephioParentPrefixLengthKey]
	if ok {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return p
}

func (r *Allocation) GetPrefixLengthFromRoute(route *table.Route) uint8 {
	if r.PrefixLength != 0 {
		return r.PrefixLength
	}
	if route.IPPrefix().IP().Is4() {
		return 32
	}
	return 128
}

func BuildAllocationFromIPPrefix(cr *ipamv1alpha1.IPPrefix) *Allocation {
	p := netaddr.MustParseIPPrefix(cr.Spec.Prefix)
	return &Allocation{
		NamespacedName: types.NamespacedName{
			Name:      cr.GetName(),
			Namespace: cr.GetNamespace(),
		},
		Origin:          ipamv1alpha1.OriginIPPrefix,
		NetworkInstance: cr.Spec.NetworkInstance,
		PrefixKind:      ipamv1alpha1.PrefixKind(cr.Spec.PrefixKind),
		AddresFamily:    iputil.GetAddressFamily(p),
		Prefix:          cr.Spec.Prefix,
		PrefixLength:    uint8(iputil.GetPrefixLengthAsInt(p)),
		Network:         cr.Spec.Network,
		Labels:          cr.GetLabels(),
	}
}

func BuildAllocationFromIPAllocation(cr *ipamv1alpha1.IPAllocation) *Allocation {
	return &Allocation{
		NamespacedName: types.NamespacedName{
			Name:      cr.GetName(),
			Namespace: cr.GetNamespace(),
		},
		Origin:          ipamv1alpha1.OriginIPAllocation,
		NetworkInstance: cr.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey],
		PrefixKind:      ipamv1alpha1.PrefixKind(cr.Spec.PrefixKind),
		AddresFamily:    ipamv1alpha1.AddressFamily(cr.Spec.AddressFamily),
		Prefix:          cr.Spec.Prefix,
		PrefixLength:    cr.Spec.PrefixLength,
		Network:         cr.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkNameKey],
		Labels:          cr.GetLabels(),
		SelectorLabels:  cr.Spec.Selector.MatchLabels,
	}
}

func BuildAllocationFromGRPCAlloc(alloc *allocpb.Request) *Allocation {
	return &Allocation{
		NamespacedName: types.NamespacedName{
			Name:      alloc.Name,
			Namespace: alloc.Namespace,
		},
		Origin:          ipamv1alpha1.OriginIPAllocation,
		NetworkInstance: alloc.GetSpec().GetSelector()[ipamv1alpha1.NephioNetworkInstanceKey],
		PrefixKind:      ipamv1alpha1.PrefixKind(alloc.GetSpec().GetPrefixkind()),
		AddresFamily:    ipamv1alpha1.AddressFamily(alloc.GetSpec().GetAddressFamily()),
		Prefix:          alloc.GetSpec().GetPrefix(),
		PrefixLength:    uint8(alloc.GetSpec().GetPrefixLength()),
		Network:         alloc.GetSpec().GetSelector()[ipamv1alpha1.NephioNetworkNameKey],
		Labels:          alloc.GetLabels(),
		SelectorLabels:  alloc.GetSpec().GetSelector(),
	}
}

func (in *Allocation) DeepCopy() (*Allocation, error) {
	if in == nil {
		return nil, errors.New("in cannot be nil")
	}
	//fmt.Printf("json copy input %v\n", in)
	bytes, err := json.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal input data")
	}
	out := &Allocation{}
	err = json.Unmarshal(bytes, &out)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal to output data")
	}
	return out, nil
}
