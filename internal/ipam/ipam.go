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
	"sync"

	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Option can be used to manipulate Options.
type Option func(Ipam)

type Ipam interface {
	// Create and initialize the IPAM instance
	Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
	// Delete the ipam instance
	Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance)
	// Add a dynamic watch with callback to the ipam rib
	AddWatch(ownerGvkKey, ownerGvk string, fn CallbackFn)
	// Delete a dynamic watch with callback to the ipam rib
	DeleteWatch(ownerGvkKey, ownerGvk string)
	// AllocateIPPrefix allocates an ip prefix
	AllocateIPPrefix(ctx context.Context, cr *ipamv1alpha1.IPAllocation) (*ipamv1alpha1.IPAllocation, error)
	// DeAllocateIPPrefix
	DeAllocateIPPrefix(ctx context.Context, cr *ipamv1alpha1.IPAllocation) error
}

func New(c client.Client, opts ...Option) Ipam {
	ipamRib := newIpamRib()
	i := &ipam{
		ipamRib:       ipamRib,
		ipamOperation: NewIPamOperation(&IPAMOperationMapConfig{ipamRib: ipamRib}),
		c:             c,
		watches: make(map[string]*watchContext),
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

type ipam struct {
	c client.Client
	m sync.RWMutex
	watches map[string]*watchContext
	ipamRib       ipamRib
	ipamOperation IPAMOperations

	l logr.Logger
}

func (r *ipam) AddWatch(ownerGvkKey, ownerGvk string, fn CallbackFn) {
	r.addWatch(ownerGvkKey, ownerGvk, fn)
}
func (r *ipam) DeleteWatch(ownerGvkKey, ownerGvk string) {
	r.deleteWatch(ownerGvkKey, ownerGvk)
}

// Initialize and create the ipam instance with the allocated prefixes
func (r *ipam) Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	r.l = log.FromContext(context.Background())

	// if the IPAM is not initialaized initialaize it
	// this happens upon initialization or ipam restart
	if r.ipamRib.init(cr.GetName()) {
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
			if labels.Get(ipamv1alpha1.NephioOwnerGvkKey) == ipamv1alpha1.NetworkInstanceGVKString {
				for _, ipprefix := range cr.Spec.Prefixes {
					// the prefix is implicitly checked based on the name
					if labels.Get(ipamv1alpha1.NephioNsnKey) == cr.GetNameFromNetworkInstancePrefix(ipprefix.Prefix) {
						if prefix != ipprefix.Prefix {
							r.l.Error(fmt.Errorf("strange that the prefixes dont match: ipam prefix: %s, spec ipprefix: %s", prefix, ipprefix.Prefix),
								"mismatch prefixes", "kind", "network-instance aggregate")
						}

						alloc := ipamv1alpha1.BuildIPAllocationFromNetworkInstancePrefix(cr, ipprefix)
						op, err := r.ipamOperation.GetPrefixOperation().Get(alloc)
						if err != nil {
							r.l.Error(err, "canot initialize ipam operation map")
							return err
						}
						if _, err := op.Apply(ctx); err != nil {
							r.l.Error(err, "canot apply aggregate prefix from network instance on init")
							return err
						}

						/*
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
						*/
					}
				}
			}
		}

		for prefix, labels := range cr.Status.Allocations {
			if labels.Get(ipamv1alpha1.NephioOwnerGvkKey) == ipamv1alpha1.IPPrefixKindGVKString {
				r.l.Info("ipam action", "action", "initialize", "prefix", prefix, "labels", labels)
				for _, ipprefix := range prefixList.Items {
					r.l.Info("ipam action", "action", "initialize", "ipprefix", ipprefix)
					if labels.Get(ipamv1alpha1.NephioNsnKey) == ipprefix.Name {
						if prefix != ipprefix.Spec.Prefix {
							r.l.Error(fmt.Errorf("strange that the prefixes dont match: ipam prefix: %s, spec ipprefix: %s", prefix, ipprefix.Spec.Prefix),
								"mismatch prefixes", "kind", ipprefix.Spec.PrefixKind)
						}

						alloc := ipamv1alpha1.BuildIPAllocationFromIPPrefix(&ipprefix)
						op, err := r.ipamOperation.GetPrefixOperation().Get(alloc)
						if err != nil {
							r.l.Error(err, "canot initialize ipam operation map")
							return err
						}
						if _, err := op.Apply(ctx); err != nil {
							r.l.Error(err, "canot apply aggregate prefix from network instance on init")
							return err
						}

						/*
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
						*/
					}
				}
			}
		}

		for prefix, labels := range cr.Status.Allocations {
			if labels.Get(ipamv1alpha1.NephioOwnerGvkKey) == ipamv1alpha1.IPAllocationKindGVKString {
				r.l.Info("ipam action", "action", "initialize", "prefix", prefix, "labels", labels)
				for _, ipalloc := range allocList.Items {
					r.l.Info("ipam action", "action", "initialize", "ipalloc", ipalloc)
					if labels.Get(ipamv1alpha1.NephioNsnKey) == ipalloc.Name {
						if prefix != ipalloc.Status.AllocatedPrefix {
							r.l.Error(fmt.Errorf("strange that the prefixes dont match: ipam prefix: %s, spec ipprefix: %s", prefix, ipalloc.Spec.Prefix),
								"mismatch prefixes", "kind", ipalloc.Spec.PrefixKind)
						}

						op, err := r.ipamOperation.GetPrefixOperation().Get(&ipalloc)
						if err != nil {
							r.l.Error(err, "canot initialize ipam operation map")
							return err
						}
						if _, err := op.Apply(ctx); err != nil {
							r.l.Error(err, "canot apply aggregate prefix from network instance on init")
							return err
						}

						/*
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
						*/
					}
				}
			}
		}

		r.l.Info("ipam create instance done")
		return r.ipamRib.initDone(cr.GetName())
	}

	return nil
}

// Delete the ipam instance
func (r *ipam) Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) {
	r.m.Lock()
	defer r.m.Unlock()
	r.l = log.FromContext(context.Background())
	r.l.Info("ipam action", "action", "delete", "name", cr.GetName())
	r.ipamRib.delete(cr.GetName())
	//delete(r.ipam, cr.GetName())
}

// AllocateIPPrefix allocates the prefix
func (r *ipam) AllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("allocate prefix ", "alloc", alloc)

	// get the ipam operation based the following parameters
	// prefixkind
	// hasprefix -> if prefix parsing is nok we return an error
	// networkinstance -> if not initialized we get an error
	// initialized with alloc, rib and prefix if present
	op, err := r.getOperation(alloc)
	if err != nil {
		return nil, err
	}
	msg, err := op.Validate(ctx)
	if err != nil {
		r.l.Error(err, "validation failed")
		return nil, err
	}
	if msg != "" {
		r.l.Error(fmt.Errorf("%s", msg), "validation failed")
		return nil, fmt.Errorf("validated failed: %s", msg)
	}
	updatedAlloc, err := op.Apply(ctx)
	if err != nil {
		return nil, err
	}
	r.l.Info("allocate prefix done", "updatedAlloc", updatedAlloc)
	return updatedAlloc, r.updateNetworkInstanceStatus(ctx, alloc)
}

func (r *ipam) DeAllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error {
	r.l = log.FromContext(ctx)

	// get the ipam operation based the following parameters
	// prefixkind
	// hasprefix -> if prefix parsing is nok we return an error
	// networkinstance -> if not initialized we get an error
	// initialized with alloc, rib and prefix if present
	op, err := r.getOperation(alloc)
	if err != nil {
		r.l.Error(err, "cannot get ipam operation map")
		return err
	}
	if err := op.Delete(ctx); err != nil {
		r.l.Error(err, "cannot deallocate prefix")
		return err
	}
	return r.updateNetworkInstanceStatus(ctx, alloc)
}

func (r *ipam) updateNetworkInstanceStatus(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error {
	rib, err := r.ipamRib.getRIB(alloc.GetNetworkInstance(), false)
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
	for _, route := range rib.GetTable() {
		r.l.Info("updateNetworkInstanceStatus insertor", "route", route.String())
		// TBD -< right now i map the data into the labels for  simplicity -> TBD
		labels := route.Labels()
		for k := range route.GetData() {
			r.l.Info("updateNetworkInstanceStatus insertor", "key", k)
			labels[k] = ""
		}
		r.l.Info("updateNetworkInstanceStatus insertor", "route", route.String())
		ni.Status.Allocations[route.String()] = labels

	}
	return errors.Wrap(r.c.Status().Update(ctx, ni), "cannot update ni status")
}

func (r *ipam) getOperation(alloc *ipamv1alpha1.IPAllocation) (IPAMOperation, error) {
	if alloc.GetPrefix() == "" {
		return r.ipamOperation.GetAllocOperation().Get(alloc)
	}
	return r.ipamOperation.GetPrefixOperation().Get(alloc)
}
