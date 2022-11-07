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
	"strconv"
	"strings"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
)

type MutatorFn func(alloc *Allocation) []*Allocation

func (r *ipam) mutateAllocation(alloc *Allocation) []*Allocation {
	r.mm.Lock()
	mutatFn := r.mutator[ipamUsage{PrefixKind: alloc.PrefixKind, HasPrefix: alloc.Prefix != ""}]
	r.mm.Unlock()

	//ipamallocs := []*Allocation{}

	//allocs :=  mutatFn(alloc)
	/*
		for _, alloc := range allocs {
			ipamallocs = append(ipamallocs, alloc.buildNewAllocation())
		}
	*/
	return mutatFn(alloc)
}

func (r *ipam) nopMutator(alloc *Allocation) []*Allocation {
	return []*Allocation{}
}

func (r *ipam) genericMutatorWithoutPrefix(alloc *Allocation) []*Allocation {
	newallocs := []*Allocation{}

	// copy allocation
	newalloc, _ := alloc.DeepCopy()
	//*newalloc = *alloc

	newlabels := newalloc.GetLabels()
	newlabels[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	// Add prefix kind key
	newlabels[ipamv1alpha1.NephioPrefixKindKey] = string(newalloc.GetPrefixKind())
	// Add address family key
	newlabels[ipamv1alpha1.NephioAddressFamilyKey] = string(newalloc.GetAddressFamily())

	// add ip pool labelKey if present
	if newalloc.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
		newlabels[ipamv1alpha1.NephioPoolKey] = "true"
	}

	// NO GW allowed here
	delete(newlabels, ipamv1alpha1.NephioGatewayKey)

	newlabels[ipamv1alpha1.NephioIPPrefixNameKey] = alloc.GetName()
	newlabels[ipamv1alpha1.NephioOriginKey] = string(ipamv1alpha1.OriginIPPrefix)
	if alloc.PrefixLength != 0 {
		newlabels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(int(alloc.PrefixLength))
	}

	newalloc.Labels = newlabels

	selectorLabels := newalloc.GetSelectorLabels()
	selectorLabels[ipamv1alpha1.NephioNetworkInstanceKey] = alloc.GetNetworkInstance()
	newalloc.SelectorLabels = selectorLabels

	// build the generic allocation
	newallocs = append(newallocs, newalloc)

	return newallocs
}

// genericMutator mutates the allocation
// removes gateway key in the label
func (r *ipam) genericMutatorWithPrefix(alloc *Allocation) []*Allocation {
	newallocs := []*Allocation{}

	// copy allocation
	newalloc, _ := alloc.DeepCopy()
	//*newalloc = *alloc

	newlabels := newalloc.GetLabels()
	newlabels[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	newlabels[ipamv1alpha1.NephioPrefixKindKey] = string(newalloc.GetPrefixKind())
	newlabels[ipamv1alpha1.NephioAddressFamilyKey] = string(newalloc.GetAddressFamily())
	// if the AF is unknown we derive it from the prefix
	if newalloc.GetAddressFamily() == ipamv1alpha1.AddressFamilyUnknown {
		newlabels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(alloc.GetIPPrefix()))
	}
	// add ip pool labelKey if present
	if newalloc.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
		newlabels[ipamv1alpha1.NephioPoolKey] = "true"
	}

	// NO GW allowed here
	delete(newlabels, ipamv1alpha1.NephioGatewayKey)

	newlabels[ipamv1alpha1.NephioIPPrefixNameKey] = alloc.GetName()
	newlabels[ipamv1alpha1.NephioOriginKey] = string(ipamv1alpha1.OriginIPPrefix)
	newlabels[ipamv1alpha1.NephioPrefixLengthKey] = iputil.GetPrefixLength(alloc.GetIPPrefix())
	newalloc.Labels = newlabels

	selectorLabels := newalloc.GetSelectorLabels()
	selectorLabels[ipamv1alpha1.NephioNetworkInstanceKey] = alloc.GetNetworkInstance()
	newalloc.SelectorLabels = selectorLabels

	// build the generic allocation
	newallocs = append(newallocs, newalloc)

	return newallocs
}

func (r *ipam) networkMutator(alloc *Allocation) []*Allocation {
	newallocs := []*Allocation{}

	r.l.Info("networkMutator", "alloc", alloc)

	// prepare additional labels generically
	p := alloc.GetIPPrefix()
	newlabels := alloc.GetLabels()
	newlabels[ipamv1alpha1.NephioPrefixKindKey] = string(alloc.GetPrefixKind())
	newlabels[ipamv1alpha1.NephioOriginKey] = string(alloc.GetOrigin())
	newlabels[ipamv1alpha1.NephioAddressFamilyKey] = string(alloc.GetAddressFamily())
	if alloc.GetAddressFamily() == ipamv1alpha1.AddressFamilyUnknown {
		newlabels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(p))
	}
	newlabels[ipamv1alpha1.NephioNetworkNameKey] = alloc.GetNetwork()
	// NO POOL allowed here
	delete(newlabels, ipamv1alpha1.NephioPoolKey)
	alloc.Labels = newlabels

	selectorLabels := alloc.GetSelectorLabels()
	selectorLabels[ipamv1alpha1.NephioNetworkInstanceKey] = alloc.GetNetworkInstance()
	alloc.SelectorLabels = selectorLabels

	//newalloc, _ := alloc.DeepCopy()
	if p.IPNet().String() != p.Masked().String() {
		// allocate the address
		newallocs = append(newallocs, r.networkAddressMutator(alloc))
		// allocate the network
		newallocs = append(newallocs, r.networkNetMutator(alloc))
		// allocate the first address)
		newallocs = append(newallocs, r.networkFirstMutator(alloc))
		// allocate the last address
		// TODO

	} else {
		// allocate the address part
		newallocs = append(newallocs, r.networkAddressMutator(alloc))
		// allocate the network part
		newallocs = append(newallocs, r.networkNetMutator(alloc))
	}

	return newallocs
}

func (r *ipam) networkAddressMutator(alloc *Allocation) *Allocation {
	// copy allocation
	r.l.Info("networkAddressMutator before", "alloc", alloc)
	newalloc, _ := alloc.DeepCopy()
	r.l.Info("networkAddressMutator after", "alloc", newalloc)
	//*newalloc = *alloc

	p := alloc.GetIPPrefix()
	newalloc.NamespacedName.Name = alloc.NamespacedName.Name
	newalloc.Prefix = iputil.GetAddress(p)

	newlabels := newalloc.GetLabels()
	newlabels[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	newlabels[ipamv1alpha1.NephioIPPrefixNameKey] = newalloc.GetName()
	newlabels[ipamv1alpha1.NephioNetworkKey] = p.Masked().IP().String()
	newlabels[ipamv1alpha1.NephioPrefixLengthKey] = iputil.GetAddressPrefixLength(p)
	newlabels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.GetPrefixLength(p)
	newalloc.Labels = newlabels

	r.l.Info("networkAddressMutator end", "alloc", newalloc)
	return newalloc
}

func (r *ipam) networkNetMutator(alloc *Allocation) *Allocation {
	// copy allocation
	newalloc, _ := alloc.DeepCopy()
	//*newalloc = *alloc

	p := alloc.GetIPPrefix()
	newalloc.NamespacedName.Name = strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-")
	newalloc.Prefix = p.Masked().String()

	newlabels := newalloc.GetLabels()
	// NO GW allowed here
	delete(newlabels, ipamv1alpha1.NephioGatewayKey)
	newlabels[ipamv1alpha1.NephioIPAllocactionNameKey] = strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-")
	newlabels[ipamv1alpha1.NephioOriginKey] = "system"
	newlabels[ipamv1alpha1.NephioIPPrefixNameKey] = "net"
	newlabels[ipamv1alpha1.NephioNetworkKey] = p.Masked().IP().String()
	newlabels[ipamv1alpha1.NephioPrefixLengthKey] = iputil.GetPrefixLength(p)
	newalloc.Labels = newlabels

	return newalloc
}

func (r *ipam) networkFirstMutator(alloc *Allocation) *Allocation {
	// copy allocation

	newalloc, _ := alloc.DeepCopy()
	//*newalloc = *alloc

	p := alloc.GetIPPrefix()
	newalloc.NamespacedName.Name = p.Masked().IP().String()
	newalloc.Prefix = iputil.GetFirstAddress(p)

	newlabels := newalloc.GetLabels()
	// NO GW allowed here
	delete(newlabels, ipamv1alpha1.NephioGatewayKey)
	newlabels[ipamv1alpha1.NephioIPAllocactionNameKey] = p.Masked().IP().String()
	newlabels[ipamv1alpha1.NephioOriginKey] = "system"
	newlabels[ipamv1alpha1.NephioIPPrefixNameKey] = "net"
	newlabels[ipamv1alpha1.NephioNetworkKey] = p.Masked().IP().String()
	newlabels[ipamv1alpha1.NephioPrefixLengthKey] = iputil.GetAddressPrefixLength(p)
	newlabels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.GetPrefixLength(p)
	newalloc.Labels = newlabels

	return newalloc
}
