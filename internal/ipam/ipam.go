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
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/pkg/errors"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Option can be used to manipulate Options.
type Option func(Ipam)

type Ipam interface {
	// Init
	Init(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
	// Delete the ipam instance
	Delete(string)
	// AllocateIPPrefix allocates an ip prefix
	AllocateIPPrefix(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error)
	// DeAllocateIPPrefix
	DeAllocateIPPrefix(ctx context.Context, alloc *Allocation) error
}

func New(c client.Client, opts ...Option) Ipam {
	i := &ipam{
		c:    c,
		ipam: make(map[string]*table.RouteTable),
	}

	i.validator = map[ipamUsage]*ValidationConfig{
		// Allocation has a prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputNetworkWithPrefixFn,
			IsAddressFn:        IsAddressGenericFn,
			IsAddressInNetFn:   IsAddressInNetNopFn,
			ExactPrefixMatchFn: ExactPrefixMatchNetworkFn,
			ChildrenExistFn:    ChildrenExistGenericFn,
			ParentExistFn:      ParentExistNetworkFn,
			FinalValidationFn:  FinalValidationNetworkFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputNopFn,
			IsAddressFn:        IsAddressNopFn,
			IsAddressInNetFn:   IsAddressInNetGenericFn,
			ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
			ChildrenExistFn:    ChildrenExistLoopbackFn,
			ParentExistFn:      ParentExistLoopbackFn,
			FinalValidationFn:  FinalValidationNopFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindPool, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputNopFn,
			IsAddressFn:        IsAddressGenericFn,
			IsAddressInNetFn:   IsAddressInNetGenericFn,
			ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
			ChildrenExistFn:    ChildrenExistGenericFn,
			ParentExistFn:      ParentExistPoolFn,
			FinalValidationFn:  FinalValidationNopFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindAggregate, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputNopFn,
			IsAddressFn:        IsAddressGenericFn,
			IsAddressInNetFn:   IsAddressInNetGenericFn,
			ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
			ChildrenExistFn:    ChildrenExistNopFn,
			ParentExistFn:      ParentExistAggregateFn,
			FinalValidationFn:  FinalValidationNopFn,
		},
		// Allocation has no prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: false}: {
			ValidateInputFn: ValidateInputNetworkWithoutPrefixFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: false}: {
			ValidateInputFn: ValidateInputNopFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindPool, HasPrefix: false}: {
			ValidateInputFn: ValidateInputNopFn,
		},
		// aggregate prefixes should always have a prefix
		{PrefixKind: ipamv1alpha1.PrefixKindAggregate, HasPrefix: false}: {
			ValidateInputFn: ValidateInputNopFn,
		},
	}

	i.mutator = map[ipamUsage]MutatorFn{
		// Allocation has a prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: true}:   i.networkMutator,
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: true}:  i.genericMutatorWithPrefix,
		{PrefixKind: ipamv1alpha1.PrefixKindPool, HasPrefix: true}:      i.genericMutatorWithPrefix,
		{PrefixKind: ipamv1alpha1.PrefixKindAggregate, HasPrefix: true}: i.genericMutatorWithPrefix,
		// Allocation has no prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: false}:  i.genericMutatorWithoutPrefix,
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: false}: i.genericMutatorWithoutPrefix,
		{PrefixKind: ipamv1alpha1.PrefixKindPool, HasPrefix: false}:     i.genericMutatorWithoutPrefix,
		// aggregate prefixes should always have a prefix
		{PrefixKind: ipamv1alpha1.PrefixKindAggregate, HasPrefix: false}: i.nopMutator,
	}

	i.insertor = map[ipamUsage]InsertorFn{
		// Allocation has a prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: true}:   i.GenericPrefixInsertor,
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: true}:  i.GenericPrefixInsertor,
		{PrefixKind: ipamv1alpha1.PrefixKindPool, HasPrefix: true}:      i.GenericPrefixInsertor,
		{PrefixKind: ipamv1alpha1.PrefixKindAggregate, HasPrefix: true}: i.GenericPrefixInsertor,
		// Allocation has no prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: false}:  i.GenericPrefixAllocator,
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: false}: i.GenericPrefixAllocator,
		{PrefixKind: ipamv1alpha1.PrefixKindPool, HasPrefix: false}:     i.GenericPrefixAllocator,
		// aggregate prefixes should always have a prefix
		{PrefixKind: ipamv1alpha1.PrefixKindAggregate, HasPrefix: false}: i.NopPrefixAllocator,
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

type ipam struct {
	c    client.Client
	m    sync.Mutex
	ipam map[string]*table.RouteTable

	vm        sync.RWMutex
	validator map[ipamUsage]*ValidationConfig
	mm        sync.RWMutex
	mutator   map[ipamUsage]MutatorFn
	im        sync.RWMutex
	insertor  map[ipamUsage]InsertorFn

	l logr.Logger
}

// Initialize and create the ipam instance with the allocated prefixes
func (r *ipam) Init(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	r.l = log.FromContext(context.Background())

	// if the IPAM is not initialaized initialaize it
	if _, ok := r.ipam[cr.GetNamespacedName().String()]; !ok {
		r.l.Info("ipam action", "action", "initialize", "name", cr.GetNamespacedName().String())

		r.m.Lock()
		r.ipam[cr.GetNamespacedName().String()] = table.NewRouteTable()
		r.m.Unlock()

		prefixList := &ipamv1alpha1.IPPrefixList{}
		if err := r.c.List(context.Background(), prefixList); err != nil {
			return errors.Wrap(err, "cannot get ip prefix list")
		}

		// list all allocation to restore them in the ipam upon restart
		allocList := &ipamv1alpha1.IPAllocationList{}
		if err := r.c.List(context.Background(), allocList); err != nil {
			return errors.Wrap(err, "cannot get ip allocation list")
		}
		for prefix, labels := range cr.Status.Allocations {
			switch labels.Get(ipamv1alpha1.NephioOriginKey) {
			case string(ipamv1alpha1.OriginIPPrefix):
				for _, ipprefix := range prefixList.Items {
					if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == ipprefix.Name {
						if prefix != ipprefix.Spec.Prefix {
							r.l.Info("ipam action",
								"action", "initialize",
								"prefix", "match error",
								"ni prefix", prefix,
								"ipprefix prefix", ipprefix.Spec.Prefix,
								"kind", ipprefix.Spec.PrefixKind)
						}

						r.mm.Lock()
						mutatorFn := r.mutator[ipamUsage{
							PrefixKind: ipamv1alpha1.PrefixKind(ipprefix.Spec.PrefixKind),
							HasPrefix:  true}]
						r.mm.Unlock()
						allocs := mutatorFn(BuildAllocationFromIPPrefix(&ipprefix))

						for _, alloc := range allocs {
							_, err := r.AllocateIPPrefix(ctx, alloc)
							if err != nil {
								return err
							}
							r.l.Info("ipam action",
								"action", "initialize",
								"added prefix", alloc.Prefix)
						}
					}
				}
			case string(ipamv1alpha1.OriginIPAllocation):
				for _, ipalloc := range allocList.Items {
					if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == ipalloc.Name {
						if prefix != ipalloc.Spec.Prefix {
							r.l.Info("ipam action", "action", "initialize", "alloc", "match error", "ni prefix", prefix, "alloc prefix", ipalloc.Spec.Prefix)
						}

						r.mm.Lock()
						mutatorFn := r.mutator[ipamUsage{
							PrefixKind: ipamv1alpha1.PrefixKind(ipalloc.Spec.PrefixKind),
							HasPrefix:  ipalloc.Spec.Prefix != ""}]
						r.mm.Unlock()
						allocs := mutatorFn(BuildAllocationFromIPAllocation(&ipalloc))

						for _, alloc := range allocs {
							_, err := r.AllocateIPPrefix(ctx, alloc)
							if err != nil {
								return err
							}
							r.l.Info("ipam action", "action", "initialize", "added prefix", alloc.Prefix)
						}

						r.l.Info("ipam action", "action", "initialize", "added alloc", prefix)
					}
				}
			default:
			}
		}

	}

	return nil
}

// Delete the ipam instance
func (r *ipam) Delete(crName string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.l = log.FromContext(context.Background())
	r.l.Info("ipam action", "action", "delete", "name", crName)
	delete(r.ipam, crName)
}

// AllocateIPPrefix allocates the prefix
func (r *ipam) AllocateIPPrefix(ctx context.Context, alloc *Allocation) (*AllocatedPrefix, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("allocate prefix ", "alloc", alloc)

	// copy original allocation
	origAlloc := new(Allocation)
	*origAlloc = *alloc

	// validate alloc
	msg, err := r.validate(ctx, alloc)
	if err != nil {
		return nil, err
	}
	if msg != "" {
		return nil, fmt.Errorf("validated failed: %s", msg)
	}

	// mutate alloc from Allocation to []IpamAllocation
	allocs := r.mutateAllocation(alloc)

	// insert alloc in ipam
	allocatedPrefix := &AllocatedPrefix{}
	for _, alloc := range allocs {
		r.l.Info("applyAllocation", "alloc", alloc)
		ap, err := r.applyAllocation(ctx, alloc)
		if err != nil {
			return nil, err
		}
		if origAlloc.GetName() == alloc.GetName() {
			allocatedPrefix = ap
		}
	}
	return allocatedPrefix, r.updateNetworkInstanceStatus(ctx, origAlloc)
}

func (r *ipam) DeAllocateIPPrefix(ctx context.Context, alloc *Allocation) error {
	// copy original allocation
	origAlloc := new(Allocation)
	*origAlloc = *alloc
	r.l.Info("deallocate prefix ", "alloc", alloc)

	rt, err := r.getRoutingTable(alloc, false)
	if err != nil {
		return err
	}

	r.mm.Lock()
	mutatorFn := r.mutator[ipamUsage{
		PrefixKind: alloc.PrefixKind,
		HasPrefix:  alloc.Prefix != ""}]
	r.mm.Unlock()
	allocs := mutatorFn(alloc)
	if !r.IsLatestPrefixInNetwork(alloc) {
		r.l.Info("deallocate prefix ", "latest", "false")
		allocs = allocs[:1]
	}
	for _, alloc := range allocs {
		r.l.Info("deallocate individual prefix ", "alloc", alloc)

		allocSelector, err := alloc.GetAllocSelector()
		if err != nil {
			return err
		}
		r.l.Info("deallocate individual prefix", "allocSelector", allocSelector)

		routes := rt.GetByLabel(allocSelector)
		for _, route := range routes {
			_, ok, err := rt.Delete(route)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("prefix not deleted")
			}
		}
	}

	return r.updateNetworkInstanceStatus(ctx, origAlloc)
}

func (r *ipam) get(crName string) (*table.RouteTable, bool) {
	r.m.Lock()
	defer r.m.Unlock()
	rt, ok := r.ipam[crName]
	return rt, ok
}

func (r *ipam) getRoutingTable(alloc *Allocation, dryrun bool) (*table.RouteTable, error) {
	rt, ok := r.get(alloc.GetNINamespacedName().String())
	if !ok {
		return nil, fmt.Errorf("ipam ni not ready or network-instance %s not correct", alloc.GetNINamespacedName().String())
	}
	if dryrun {
		// copy the routing table for validation
		return copyRoutingTable(rt), nil
	}
	//r.l.Info("validateIPAllocation", "ipam size", rt.Size())
	return rt, nil
}

func copyRoutingTable(rt *table.RouteTable) *table.RouteTable {
	newrt := table.NewRouteTable()
	for _, route := range rt.GetTable() {
		newrt.Add(route)
	}
	return newrt
}

func (r *ipam) updateNetworkInstanceStatus(ctx context.Context, alloc *Allocation) error {
	rt, err := r.getRoutingTable(alloc, false)
	if err != nil {
		return err
	}

	// update allocations based on latest routing table
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.c.Get(ctx, alloc.GetNINamespacedName(), ni); err != nil {
		return errors.Wrap(err, "cannot get network instance")
	}

	// always reinitialize the allocations based on latest info
	ni.Status.Allocations = make(map[string]labels.Set)
	for _, route := range rt.GetTable() {
		//r.l.Info("updateNetworkInstanceStatus", "route", route.String())
		ni.Status.Allocations[route.String()] = *route.GetLabels()
	}
	return errors.Wrap(r.c.Status().Update(ctx, ni), "cannot update ni status")
}

func (r *ipam) IsLatestPrefixInNetwork(alloc *Allocation) bool {
	// only relevant for prefixkind network and allocations with prefixes
	if alloc.PrefixKind != ipamv1alpha1.PrefixKindNetwork ||
		alloc.Prefix == "" {
		r.l.Info("deallocate prefix ", "IsLatestPrefixInNetwork", "true")
		return true
	}
	r.l.Info("deallocate prefix ", "IsLatestPrefixInNetwork", "tbd", "alloc", alloc)
	rt, err := r.getRoutingTable(alloc, false)
	if err != nil {
		// this should never occur, if so the route should no longre be there
		return false
	}

	// get all routes with the network label
	l, _ := alloc.GetNetworkLabelSelector()
	routes := rt.GetByLabel(l)
	r.l.Info("deallocate prefix ", "IsLatestPrefixInNetwork", "tbd", "labelSelector", l)

	// for each route that belongs to the network
	p := netaddr.MustParseIPPrefix(alloc.Prefix)
	totalCount := len(routes)
	validCount := 0
	for _, route := range routes {
		// compare against the allocation name -> net routes, first route and real prefix
		switch route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey) {
		case alloc.GetName(),
			p.Masked().IP().String(),
			strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-"):
			validCount++
		}
		r.l.Info("is latest prefix in network",
			"allocationName", route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey),
			"netName", strings.Join([]string{p.Masked().IP().String(), iputil.GetPrefixLength(p)}, "-"),
			"totalCount", totalCount,
			"validCount", validCount,
		)
	}
	r.l.Info("deallocate prefix ", "IsLatestPrefixInNetwork", totalCount == validCount, "totoal", totalCount, "valid", validCount)
	return totalCount == validCount

}
