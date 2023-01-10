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
	"github.com/pkg/errors"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Option can be used to manipulate Options.
type Option func(Ipam)

type Ipam interface {
	// Init
	Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
	// Delete the ipam instance
	Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance)
	// AllocateIPPrefix allocates an ip prefix
	AllocateIPPrefix(ctx context.Context, cr *ipamv1alpha1.IPAllocation) (*ipamv1alpha1.IPAllocation, error)
	// DeAllocateIPPrefix
	DeAllocateIPPrefix(ctx context.Context, cr *ipamv1alpha1.IPAllocation) error
}

func New(c client.Client, opts ...Option) Ipam {
	i := &ipam{
		c:    c,
		ipam: make(map[string]*ipamInfo),
	}

	i.validator = map[ipamUsage]*ValidationConfig{
		// Allocation has a prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputGenericWithPrefixFn,
			IsAddressFn:        IsAddressGenericFn,
			IsAddressInNetFn:   IsAddressInNetNopFn,
			ExactMatchPrefixFn: ExactMatchPrefixNetworkFn,
			ExactPrefixMatchFn: ExactPrefixMatchNetworkFn,
			ChildrenExistFn:    ChildrenExistGenericFn,
			NoParentExistFn:    NoParentExistGenericFn,
			ParentExistFn:      ParentExistNetworkFn,
			FinalValidationFn:  FinalValidationNetworkFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputGenericWithPrefixFn,
			IsAddressFn:        IsAddressNopFn,
			IsAddressInNetFn:   IsAddressInNetGenericFn,
			ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
			ChildrenExistFn:    ChildrenExistLoopbackFn,
			NoParentExistFn:    NoParentExistGenericFn,
			ParentExistFn:      ParentExistLoopbackFn,
			FinalValidationFn:  FinalValidationNopFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindPool, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputNopFn,
			IsAddressFn:        IsAddressNopFn,
			IsAddressInNetFn:   IsAddressInNetGenericFn,
			ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
			ChildrenExistFn:    ChildrenExistGenericFn,
			NoParentExistFn:    NoParentExistGenericFn,
			ParentExistFn:      ParentExistPoolFn,
			FinalValidationFn:  FinalValidationNopFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindAggregate, HasPrefix: true}: {
			ValidateInputFn:    ValidateInputNopFn,
			IsAddressFn:        IsAddressGenericFn,
			IsAddressInNetFn:   IsAddressInNetGenericFn,
			ExactPrefixMatchFn: ExactPrefixMatchGenericFn,
			ChildrenExistFn:    ChildrenExistNopFn,
			NoParentExistFn:    NoParentExistAggregateFn,
			ParentExistFn:      ParentExistAggregateFn,
			FinalValidationFn:  FinalValidationNopFn,
		},
		// Allocation has no prefix
		{PrefixKind: ipamv1alpha1.PrefixKindNetwork, HasPrefix: false}: {
			ValidateInputFn: ValidateInputGenericWithoutPrefixFn,
		},
		{PrefixKind: ipamv1alpha1.PrefixKindLoopback, HasPrefix: false}: {
			ValidateInputFn: ValidateInputGenericWithoutPrefixFn,
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
	ipam map[string]*ipamInfo

	vm        sync.RWMutex
	validator map[ipamUsage]*ValidationConfig
	mm        sync.RWMutex
	mutator   map[ipamUsage]MutatorFn
	im        sync.RWMutex
	insertor  map[ipamUsage]InsertorFn

	l logr.Logger
}

type ipamInfo struct {
	init bool
	rt   *table.RouteTable
}

// Initialize and create the ipam instance with the allocated prefixes
func (r *ipam) Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	r.l = log.FromContext(context.Background())

	// if the IPAM is not initialaized initialaize it
	// this happens upon initialization or ipam restart
	if r.init(cr.GetName()) {
		r.l.Info("ipam create instance", "name", cr.GetName())

		// List the ip prefixes to restore the in the ipam upon restart
		prefixList := &ipamv1alpha1.IPPrefixList{}
		if err := r.c.List(context.Background(), prefixList); err != nil {
			return errors.Wrap(err, "cannot get ip prefix list")
		}

		// list all allocation to restore them in the ipam upon restart
		// this is the list of allocations that uses the k8s API
		allocList := &ipamv1alpha1.IPAllocationList{}
		if err := r.c.List(context.Background(), allocList); err != nil {
			return errors.Wrap(err, "cannot get ip allocation list")
		}

		// INFO: dynamic allocations which dont come throught the k8s api
		// are not resstored, we assume the grpc client takes care of that
		for prefix, labels := range cr.Status.Allocations {
			if labels.Get(ipamv1alpha1.NephioOwnerGvkKey) == string(ipamv1alpha1.OriginNetworkInstance) {
				for _, ipprefix := range cr.Spec.Prefixes {
					// the prefix is implicitly check based on the name
					if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == cr.GetNameFromNetworkInstancePrefix(ipprefix.Prefix) {
						if prefix != ipprefix.Prefix {
							r.l.Error(fmt.Errorf("strange that the prefixes dont match: ipam prefix: %s, spec ipprefix: %s", prefix, ipprefix.Prefix),
								"mismatch prefixes", "kind", "network-instance aggregate")
						}
						r.mm.Lock()
						mutatorFn := r.mutator[ipamUsage{
							PrefixKind: ipamv1alpha1.PrefixKindAggregate,
							HasPrefix:  true}]
						r.mm.Unlock()
						allocs := mutatorFn(ipamv1alpha1.BuildIPAllocationFromNetworkInstancePrefix(cr, ipprefix))
						for _, alloc := range allocs {
							_, err := r.applyAllocation(ctx, alloc, true)
							//	_, err := r.AllocateIPPrefix(ctx, alloc, true)
							if err != nil {
								return err
							}
							r.l.Info("ipam action",
								"action", "initialize",
								"added prefix", alloc.GetPrefix())
						}
					}
				}
			}
		}

		for prefix, labels := range cr.Status.Allocations {
			if labels.Get(ipamv1alpha1.NephioOwnerGvkKey) == string(ipamv1alpha1.OriginIPPrefix) {
				r.l.Info("ipam action", "action", "initialize", "prefix", prefix, "labels", labels)
				for _, ipprefix := range prefixList.Items {
					r.l.Info("ipam action", "action", "initialize", "ipprefix", ipprefix)
					if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == ipprefix.Name {
						if prefix != ipprefix.Spec.Prefix {
							r.l.Error(fmt.Errorf("strange that the prefixes dont match: ipam prefix: %s, spec ipprefix: %s", prefix, ipprefix.Spec.Prefix),
								"mismatch prefixes", "kind", ipprefix.Spec.PrefixKind)
						}

						r.mm.Lock()
						mutatorFn := r.mutator[ipamUsage{
							PrefixKind: ipamv1alpha1.PrefixKind(ipprefix.Spec.PrefixKind),
							HasPrefix:  true}]
						r.mm.Unlock()
						allocs := mutatorFn(ipamv1alpha1.BuildIPAllocationFromIPPrefix(&ipprefix))

						for _, alloc := range allocs {
							_, err := r.applyAllocation(ctx, alloc, true)
							//	_, err := r.AllocateIPPrefix(ctx, alloc, true)
							if err != nil {
								return err
							}
							r.l.Info("ipam action",
								"action", "initialize",
								"added prefix", alloc.GetPrefix())
						}
					}
				}
			}
		}

		for prefix, labels := range cr.Status.Allocations {
			if labels.Get(ipamv1alpha1.NephioOwnerGvkKey) == string(ipamv1alpha1.OriginIPAllocation) {
				r.l.Info("ipam action", "action", "initialize", "prefix", prefix, "labels", labels)
				for _, ipalloc := range allocList.Items {
					r.l.Info("ipam action", "action", "initialize", "ipalloc", ipalloc)
					if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == ipalloc.Name {
						if prefix != ipalloc.Status.AllocatedPrefix {
							r.l.Error(fmt.Errorf("strange that the prefixes dont match: ipam prefix: %s, spec ipprefix: %s", prefix, ipalloc.Spec.Prefix),
								"mismatch prefixes", "kind", ipalloc.Spec.PrefixKind)
						}
						r.mm.Lock()
						mutatorFn := r.mutator[ipamUsage{
							PrefixKind: ipamv1alpha1.PrefixKind(ipalloc.Spec.PrefixKind),
							HasPrefix:  ipalloc.Spec.Prefix != ""}]
						r.mm.Unlock()
						allocs := mutatorFn(&ipalloc)

						for _, alloc := range allocs {
							_, err := r.applyAllocation(ctx, alloc, true)
							//_, err := r.AllocateIPPrefix(ctx, alloc, true)
							if err != nil {
								return err
							}
							r.l.Info("ipam action", "action", "initialize", "added prefix", alloc.GetPrefix())
						}
						r.l.Info("ipam action", "action", "initialize", "added alloc", prefix)
					}
				}
			}
		}
		r.l.Info("ipam create instance done")
		return r.initDone(cr.GetName())
	}

	return nil
}

// Delete the ipam instance
func (r *ipam) Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) {
	r.m.Lock()
	defer r.m.Unlock()
	r.l = log.FromContext(context.Background())
	r.l.Info("ipam action", "action", "delete", "name", cr.GetName())
	delete(r.ipam, cr.GetName())
}

// AllocateIPPrefix allocates the prefix
func (r *ipam) AllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("allocate prefix ", "alloc", alloc)

	// copy original allocation
	origAlloc := alloc.DeepCopy()

	// validate alloc
	msg, err := r.validate(ctx, alloc)
	if err != nil {
		r.l.Error(err, "validation failed")
		return nil, err
	}
	if msg != "" {
		r.l.Error(fmt.Errorf("%s", msg), "validation failed")
		return nil, fmt.Errorf("validated failed: %s", msg)
	}

	// mutate alloc from Allocation to []IpamAllocation
	allocs := r.mutateAllocation(alloc)

	// insert alloc in ipam
	var updatedAlloc *ipamv1alpha1.IPAllocation
	for _, alloc := range allocs {
		r.l.Info("applyAllocation", "alloc", alloc)
		ap, err := r.applyAllocation(ctx, alloc, false)
		if err != nil {
			r.l.Error(err, "applyAllocation failed")
			return nil, err
		}
		r.l.Info("allocate prefix", "name", alloc.GetName(), "prefix", alloc.GetPrefix())
		if origAlloc.GetName() == alloc.GetName() {
			updatedAlloc = ap
		}
	}
	r.l.Info("allocate prefix done", "updatedAlloc", updatedAlloc)
	return updatedAlloc, r.updateNetworkInstanceStatus(ctx, origAlloc)
}

func (r *ipam) DeAllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error {
	r.l = log.FromContext(ctx)
	// copy original allocation
	origAlloc := alloc.DeepCopy()
	r.l.Info("deallocate prefix", "alloc", alloc)

	rt, err := r.getRoutingTable(alloc, false, false)
	if err != nil {
		return err
	}

	r.mm.Lock()
	mutatorFn := r.mutator[ipamUsage{
		PrefixKind: alloc.GetPrefixKind(),
		HasPrefix:  alloc.GetPrefix() != ""}]
	r.mm.Unlock()
	allocs := mutatorFn(alloc)
	if !r.IsLatestPrefixInNetwork(alloc) {
		r.l.Info("deallocate prefix", "latest", "false")
		allocs = allocs[:1]
	}
	for _, alloc := range allocs {
		r.l.Info("deallocate individual prefix", "alloc", alloc)

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

func (r *ipam) updateNetworkInstanceStatus(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error {
	rt, err := r.getRoutingTable(alloc, false, false)
	if err != nil {
		return err
	}

	// update allocations based on latest routing table
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.c.Get(ctx, types.NamespacedName{Name: alloc.GetNetworkInstance(), Namespace: "default"}, ni); err != nil {
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

func (r *ipam) IsLatestPrefixInNetwork(alloc *ipamv1alpha1.IPAllocation) bool {
	// only relevant for prefixkind network and allocations with prefixes
	if alloc.GetPrefixKind() != ipamv1alpha1.PrefixKindNetwork ||
		alloc.GetPrefix() == "" {
		r.l.Info("deallocate prefix", "IsLatestPrefixInNetwork", "true")
		return true
	}
	r.l.Info("deallocate prefix ", "IsLatestPrefixInNetwork", "tbd", "alloc", alloc)
	rt, err := r.getRoutingTable(alloc, false, false)
	if err != nil {
		// this should never occur, if so the route should no longer be there
		return false
	}

	// get all routes with the subnet label
	l, _ := alloc.GetSubnetLabelSelector()
	routes := rt.GetByLabel(l)
	r.l.Info("deallocate prefix ", "IsLatestPrefixInNetwork", "tbd", "labelSelector", l)

	// for each route that belongs to the network
	p := netaddr.MustParseIPPrefix(alloc.GetPrefix())
	totalCount := len(routes)
	validCount := 0
	for _, route := range routes {
		// compare against the allocation name -> net routes, first route and real prefix
		switch route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey) {
		case alloc.GetName():
			validCount++
		case p.Masked().IP().String(): // this is the first address in the subnet
			validCount++
		case ipamv1alpha1.GetSubnetName(alloc.GetPrefix()): // this is the prefix in the subnet first address +
			validCount++
		}
		if route.GetLabels().Get(ipamv1alpha1.NephioOwnerGvkKey) == ipamv1alpha1.IPAllocationKindGVKString {
			validCount++
		}

		r.l.Info("is latest prefix in network",
			"allocationName", route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey),
			"netName", strings.Join([]string{p.Masked().IP().String(), ipamv1alpha1.GetPrefixLength(p)}, "-"),
			"totalCount", totalCount,
			"validCount", validCount,
		)
	}
	r.l.Info("deallocate prefix ", "IsLatestPrefixInNetwork", totalCount == validCount, "total", totalCount, "valid", validCount)
	return totalCount == validCount

}
