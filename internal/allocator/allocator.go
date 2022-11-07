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

package allocator

import (
	"fmt"
	"strings"
	"sync"

	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/utils/iputil"
	"inet.af/netaddr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Allocator interface {
	Allocate(cr *ipamv1alpha1.IPPrefix) []*ipamv1alpha1.IPAllocation
	//AllocateAlloc(alloc *ipamv1alpha1.IPAllocation) []*ipamv1alpha1.IPAllocation
}

type AllocatorFn func(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) []*ipamv1alpha1.IPAllocation

type allocator struct {
	m sync.RWMutex
	a map[ipamv1alpha1.PrefixKind]AllocatorFn
}

func New() Allocator {
	return &allocator{
		a: map[ipamv1alpha1.PrefixKind]AllocatorFn{
			ipamv1alpha1.PrefixKindNetwork:   networkAllocator,
			ipamv1alpha1.PrefixKindLoopback:  genericAllocator,
			ipamv1alpha1.PrefixKindPool:      genericAllocator,
			ipamv1alpha1.PrefixKindAggregate: genericAllocator,
		},
	}
}

func (r *allocator) Allocate(cr *ipamv1alpha1.IPPrefix) []*ipamv1alpha1.IPAllocation {
	p := netaddr.MustParseIPPrefix(cr.Spec.Prefix)

	r.m.Lock()
	defer r.m.Unlock()
	fmt.Println("Allocate", cr.Spec.PrefixKind)
	return r.a[ipamv1alpha1.PrefixKind(cr.Spec.PrefixKind)](cr, p)
}

func networkAllocator(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) []*ipamv1alpha1.IPAllocation {
	allocs := []*ipamv1alpha1.IPAllocation{}
	// for a network based prefix we have 2 options
	// a prefix where the address part of the net is equal to the net and another one where it is not
	if p.IPNet().String() != p.Masked().String() {
		// allocate the address
		allocs = append(allocs, networkAddressAllocator(cr, p))
		// allocate the network
		allocs = append(allocs, networkNetAllocator(cr, p.Masked()))
		// allocate the first address
		allocs = append(allocs, networkFirstAllocator(cr, p.Masked()))
		// allocate the last address
		// TODO

	} else {
		// allocate the address part
		allocs = append(allocs, networkAddressAllocator(cr, p))
		// allocate the network part
		allocs = append(allocs, networkNetAllocator(cr, p.Masked()))

	}
	return allocs
}

func networkNetAllocator(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) *ipamv1alpha1.IPAllocation {
	labels := getLabels(cr, p.Masked())
	// NO GW allowed here
	delete(labels, ipamv1alpha1.NephioGatewayKey)
	return buildNewAllocation(&alloc{
		Namespace: cr.GetNamespace(),
		Name:      strings.Join([]string{p.IP().String(), iputil.GetPrefixLength(p)}, "-"),
		Labels:    labels,
		SpecificLabels: map[string]string{
			ipamv1alpha1.NephioIPPrefixNameKey: "net",
			ipamv1alpha1.NephioOriginKey:       string(ipamv1alpha1.OriginIPSystem),
			ipamv1alpha1.NephioPrefixLengthKey: iputil.GetPrefixLength(p),
			ipamv1alpha1.NephioNetworkNameKey:  cr.Spec.Network,
			ipamv1alpha1.NephioNetworkKey:      p.IP().String(),
		},
		Selector:   getSelectorLabels(cr),
		Prefix:     p.Masked().String(),
		PrefixKind: cr.Spec.PrefixKind,
	})
}

func networkFirstAllocator(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) *ipamv1alpha1.IPAllocation {
	labels := getLabels(cr, p.Masked())
	// NO GW allowed here
	delete(labels, ipamv1alpha1.NephioGatewayKey)
	return buildNewAllocation(&alloc{
		Namespace: cr.GetNamespace(),
		Name:      p.IP().String(),
		Labels:    labels,
		SpecificLabels: map[string]string{
			ipamv1alpha1.NephioIPPrefixNameKey:       "net",
			ipamv1alpha1.NephioOriginKey:             string(ipamv1alpha1.OriginIPSystem),
			ipamv1alpha1.NephioPrefixLengthKey:       iputil.GetAddressPrefixLength(p),
			ipamv1alpha1.NephioParentPrefixLengthKey: iputil.GetPrefixLength(p),
			ipamv1alpha1.NephioNetworkNameKey:        cr.Spec.Network,
			ipamv1alpha1.NephioNetworkKey:            p.IP().String(),
		},
		Selector:   getSelectorLabels(cr),
		Prefix:     iputil.GetFirstAddress(p),
		PrefixKind: cr.Spec.PrefixKind,
	})
}

func networkAddressAllocator(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) *ipamv1alpha1.IPAllocation {
	return buildNewAllocation(&alloc{
		Namespace: cr.GetNamespace(),
		Name:      cr.GetName(),
		Labels:    getLabels(cr, p.Masked()),
		SpecificLabels: map[string]string{
			ipamv1alpha1.NephioIPPrefixNameKey:       cr.GetName(),
			ipamv1alpha1.NephioOriginKey:             string(ipamv1alpha1.OriginIPPrefix),
			ipamv1alpha1.NephioPrefixLengthKey:       iputil.GetAddressPrefixLength(p),
			ipamv1alpha1.NephioParentPrefixLengthKey: iputil.GetPrefixLength(p),
			ipamv1alpha1.NephioNetworkNameKey:        cr.Spec.Network,
			ipamv1alpha1.NephioNetworkKey:            p.Masked().IP().String(),
		},
		Selector:   getSelectorLabels(cr),
		Prefix:     iputil.GetAddress(p),
		PrefixKind: cr.Spec.PrefixKind,
	})
}

func genericAllocator(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) []*ipamv1alpha1.IPAllocation {

	allocs := []*ipamv1alpha1.IPAllocation{}

	labels := getLabels(cr, p)
	// NO GW allowed here
	delete(labels, ipamv1alpha1.NephioGatewayKey)

	// allocate a generic Label
	allocs = append(allocs, buildNewAllocation(&alloc{
		Namespace: cr.GetNamespace(),
		Name:      cr.GetName(),
		Labels:    labels,
		SpecificLabels: map[string]string{
			ipamv1alpha1.NephioIPPrefixNameKey: cr.GetName(),
			ipamv1alpha1.NephioOriginKey:       string(ipamv1alpha1.OriginIPPrefix),
			ipamv1alpha1.NephioPrefixLengthKey: iputil.GetPrefixLength(p),
		},
		Selector:   getSelectorLabels(cr),
		Prefix:     p.String(),
		PrefixKind: cr.Spec.PrefixKind,
	}))

	return allocs
}

func getLabels(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix) map[string]string {
	labels := cr.GetLabels()
	if len(labels) == 0 {
		labels = map[string]string{}
	}
	labels[ipamv1alpha1.NephioPrefixKindKey] = cr.Spec.PrefixKind
	// add address family to the labels associated to the prefix
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(p))
	// add ip pool labelKey if present
	if cr.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindPool) {
		labels[ipamv1alpha1.NephioPoolKey] = "true"
	}
	return labels
}

func getSelectorLabels(cr *ipamv1alpha1.IPPrefix) map[string]string {
	return map[string]string{
		ipamv1alpha1.NephioNetworkInstanceKey: cr.Spec.NetworkInstance,
	}
}

type alloc struct {
	Namespace      string
	Name           string
	Labels         map[string]string
	SpecificLabels map[string]string
	Selector       map[string]string
	Prefix         string
	PrefixKind     string
}

func buildNewAllocation(a *alloc) *ipamv1alpha1.IPAllocation {
	labels := map[string]string{}
	for k, v := range a.Labels {
		labels[k] = v
	}
	for k, v := range a.SpecificLabels {
		labels[k] = v
	}

	return &ipamv1alpha1.IPAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.Namespace,
			Name:      a.Name,
			Labels:    labels,
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			PrefixKind: a.PrefixKind,
			Prefix:     a.Prefix,
			Selector: &metav1.LabelSelector{
				MatchLabels: a.Selector,
			},
		},
	}
}
