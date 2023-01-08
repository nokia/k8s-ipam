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
)

type MutatorFn func(alloc *ipamv1alpha1.IPAllocation) []*ipamv1alpha1.IPAllocation

func (r *ipam) mutateAllocation(alloc *ipamv1alpha1.IPAllocation) []*ipamv1alpha1.IPAllocation {
	r.mm.Lock()
	mutatFn := r.mutator[ipamUsage{PrefixKind: alloc.GetPrefixKind(), HasPrefix: alloc.GetPrefix() != ""}]
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

func (r *ipam) nopMutator(alloc *ipamv1alpha1.IPAllocation) []*ipamv1alpha1.IPAllocation {
	return []*ipamv1alpha1.IPAllocation{}
}

func (r *ipam) genericMutatorWithoutPrefix(alloc *ipamv1alpha1.IPAllocation) []*ipamv1alpha1.IPAllocation {
	newallocs := []*ipamv1alpha1.IPAllocation{}

	// copy allocation
	newalloc := alloc.DeepCopy()
	//*newalloc = *alloc

	//newlabels := newalloc.GetLabels()
	if len(newalloc.Spec.Labels) == 0 {
		newalloc.Spec.Labels = map[string]string{}
	}
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	// Add prefix kind key
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixKindKey] = string(newalloc.GetPrefixKind())
	// Add address family key
	newalloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(newalloc.GetAddressFamily())

	// add ip pool labelKey if present
	if newalloc.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
		newalloc.Spec.Labels[ipamv1alpha1.NephioPoolKey] = "true"
	}

	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)

	newalloc.Spec.Labels[ipamv1alpha1.NephioIPPrefixNameKey] = alloc.GetName()
	newalloc.Spec.Labels[ipamv1alpha1.NephioOwnerKey] = alloc.GetOwner()
	if alloc.GetPrefixLength() != 0 {
		newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(int(alloc.GetPrefixLength()))
	}

	//newalloc.SpecLabels = newlabels

	//selectorLabels :=
	//newalloc.NetworkInstance = alloc.GetNetworkInstance()
	//selectorLabels[ipamv1alpha1.NephioNetworkInstanceKey] = alloc.GetNetworkInstance()
	//newalloc.SelectorLabels = newalloc.GetSelectorLabels()

	// build the generic allocation
	newallocs = append(newallocs, newalloc)

	return newallocs
}

// genericMutator mutates the allocation
// removes gateway key in the label
func (r *ipam) genericMutatorWithPrefix(alloc *ipamv1alpha1.IPAllocation) []*ipamv1alpha1.IPAllocation {
	newallocs := []*ipamv1alpha1.IPAllocation{}

	// copy allocation
	newalloc := alloc.DeepCopy()
	//*newalloc = *alloc

	//newlabels := newalloc.GetLabels()
	if len(newalloc.Spec.Labels) == 0 {
		newalloc.Spec.Labels = map[string]string{}
	}
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixKindKey] = string(newalloc.GetPrefixKind())
	newalloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(newalloc.GetAddressFamily())
	// if the AF is unknown we derive it from the prefix
	if newalloc.GetAddressFamily() == ipamv1alpha1.AddressFamilyUnknown {
		newalloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(ipamv1alpha1.GetAddressFamily(alloc.GetIPPrefix()))
	}
	// add ip pool labelKey if present
	if newalloc.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
		newalloc.Spec.Labels[ipamv1alpha1.NephioPoolKey] = "true"
	}

	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)

	newalloc.Spec.Labels[ipamv1alpha1.NephioIPPrefixNameKey] = alloc.GetName()
	newalloc.Spec.Labels[ipamv1alpha1.NephioOwnerKey] = alloc.GetOwner()
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = ipamv1alpha1.GetPrefixLength(alloc.GetIPPrefix())
	//newalloc.SpecLabels = newlabels

	//selectorLabels :=
	//selectorLabels[ipamv1alpha1.NephioNetworkInstanceKey] = alloc.GetNetworkInstance()
	//newalloc.SelectorLabels = newalloc.GetSelectorLabels()

	// build the generic allocation
	newallocs = append(newallocs, newalloc)

	return newallocs
}

func (r *ipam) networkMutator(alloc *ipamv1alpha1.IPAllocation) []*ipamv1alpha1.IPAllocation {
	newallocs := []*ipamv1alpha1.IPAllocation{}

	r.l.Info("networkMutator", "alloc", alloc)

	// prepare additional labels generically
	p := alloc.GetIPPrefix()
	//newlabels := alloc.GetLabels()
	if len(alloc.Spec.Labels) == 0 {
		alloc.Spec.Labels = map[string]string{}
	}
	alloc.Spec.Labels[ipamv1alpha1.NephioPrefixKindKey] = string(alloc.GetPrefixKind())
	alloc.Spec.Labels[ipamv1alpha1.NephioOwnerKey] = alloc.GetOwner()
	alloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(alloc.GetAddressFamily())
	if alloc.GetAddressFamily() == ipamv1alpha1.AddressFamilyUnknown {
		alloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(ipamv1alpha1.GetAddressFamily(p))
	}
	alloc.Spec.Labels[ipamv1alpha1.NephioSubnetKey] = ipamv1alpha1.GetSubnetName(alloc.GetPrefix())
	// NO POOL allowed here
	delete(alloc.Spec.Labels, ipamv1alpha1.NephioPoolKey)
	//alloc.SpecLabels = newlabels

	//selectorLabels := alloc.GetSelectorLabels()
	//selectorLabels[ipamv1alpha1.NephioNetworkInstanceKey] = alloc.GetNetworkInstance()
	//alloc.SelectorLabels = selectorLabels

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

func (r *ipam) networkAddressMutator(alloc *ipamv1alpha1.IPAllocation) *ipamv1alpha1.IPAllocation {
	// copy allocation
	r.l.Info("networkAddressMutator before", "alloc", alloc)
	newalloc := alloc.DeepCopy()
	r.l.Info("networkAddressMutator after", "alloc", newalloc)
	//*newalloc = *alloc

	p := alloc.GetIPPrefix()
	//newalloc.NamespacedName.Name = alloc.NamespacedName.Name
	newalloc.Spec.Prefix = ipamv1alpha1.GetAddress(p)

	//newlabels := newalloc.GetLabels()
	if len(newalloc.Spec.Labels) == 0 {
		newalloc.Spec.Labels = map[string]string{}
	}
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPPrefixNameKey] = newalloc.GetName()
	//newlabels[ipamv1alpha1.NephioSubnetKey] = p.Masked().IP().String()
	newalloc.Spec.Labels[ipamv1alpha1.NephioSubnetKey] = ipamv1alpha1.GetSubnetName(alloc.GetPrefix())
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = ipamv1alpha1.GetAddressPrefixLength(p)
	newalloc.Spec.Labels[ipamv1alpha1.NephioParentPrefixLengthKey] = ipamv1alpha1.GetPrefixLength(p)
	//newalloc.SpecLabels = newlabels

	r.l.Info("networkAddressMutator end", "alloc", newalloc)
	return newalloc
}

func (r *ipam) networkNetMutator(alloc *ipamv1alpha1.IPAllocation) *ipamv1alpha1.IPAllocation {
	// copy allocation
	newalloc := alloc.DeepCopy()
	//*newalloc = *alloc

	p := alloc.GetIPPrefix()
	//newalloc.NamespacedName.Name = strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-")
	newalloc.Spec.Prefix = p.Masked().String()

	//newlabels := newalloc.GetLabels()
	// NO GW allowed here
	if len(newalloc.Spec.Labels) == 0 {
		newalloc.Spec.Labels = map[string]string{}
	}
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPAllocactionNameKey] = strings.Join([]string{p.Masked().IP().String(), ipamv1alpha1.GetPrefixLength(p)}, "-")
	newalloc.Spec.Labels[ipamv1alpha1.NephioOwnerKey] = "system"
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPPrefixNameKey] = "net"
	//newlabels[ipamv1alpha1.NephioSubnetKey] = p.Masked().IP().String()
	newalloc.Spec.Labels[ipamv1alpha1.NephioSubnetKey] = ipamv1alpha1.GetSubnetName(alloc.GetPrefix())
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = ipamv1alpha1.GetPrefixLength(p)
	//newalloc.SpecLabels = newlabels

	return newalloc
}

func (r *ipam) networkFirstMutator(alloc *ipamv1alpha1.IPAllocation) *ipamv1alpha1.IPAllocation {
	// copy allocation

	newalloc := alloc.DeepCopy()
	//*newalloc = *alloc

	p := alloc.GetIPPrefix()
	//newalloc.NamespacedName.Name = p.Masked().IP().String()
	newalloc.Spec.Prefix = ipamv1alpha1.GetFirstAddress(p)

	//newlabels := newalloc.GetLabels()
	if len(newalloc.Spec.Labels) == 0 {
		newalloc.Spec.Labels = map[string]string{}
	}
	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPAllocactionNameKey] = p.Masked().IP().String()
	newalloc.Spec.Labels[ipamv1alpha1.NephioOwnerKey] = "system"
	newalloc.Spec.Labels[ipamv1alpha1.NephioIPPrefixNameKey] = "net"
	//newlabels[ipamv1alpha1.NephioSubnetKey] = p.Masked().IP().String()
	newalloc.Spec.Labels[ipamv1alpha1.NephioSubnetKey] = ipamv1alpha1.GetSubnetName(alloc.GetPrefix())
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = ipamv1alpha1.GetAddressPrefixLength(p)
	newalloc.Spec.Labels[ipamv1alpha1.NephioParentPrefixLengthKey] = ipamv1alpha1.GetPrefixLength(p)
	//newalloc.SpecLabels = newlabels

	return newalloc
}
