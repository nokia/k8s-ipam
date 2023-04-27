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
	"sync"

	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
)

type Runtimes interface {
	Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error)
}

type RuntimeConfig struct {
	cache   backend.Cache[*table.RIB]
	watcher Watcher
}

func NewRuntimes(c *RuntimeConfig) Runtimes {
	return &runtimes{
		prefixRuntime: newPrefixRuntime(c),
		allocRuntime:  newAllocRuntime(c),
	}
}

type runtimes struct {
	prefixRuntime runtime
	allocRuntime  runtime
}

func (r *runtimes) Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error) {
	if alloc.Spec.Prefix != nil {
		return r.allocRuntime.Get(alloc, initializing)
	}
	return r.prefixRuntime.Get(alloc, initializing)
}

type runtime interface {
	Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error)
}

type Runtime interface {
	Get(ctx context.Context) (*ipamv1alpha1.IPAllocation, error)
	Validate(ctx context.Context) (string, error)
	Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error)
	Delete(ctx context.Context) error
}

type PrefixRuntimeConfig struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	fnc          *PrefixValidatorFunctionConfig
	watcher      Watcher
}

type AllocRuntimeConfig struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	fnc          *AllocValidatorFunctionConfig
	watcher      Watcher
}

func newPrefixRuntime(c *RuntimeConfig) Runtimes {
	return &ipamPrefixRuntime{
		cache:   c.cache,
		watcher: c.watcher,
		oc: map[ipamv1alpha1.PrefixKind]*PrefixValidatorFunctionConfig{
			ipamv1alpha1.PrefixKindNetwork: {
				validateInputFn:         validateInput,
				validateChildrenExistFn: validateChildrenExist,
				validateNoParentExistFn: validateNoParentExist,
				validateParentExistFn:   validateParentExist,
			},
			ipamv1alpha1.PrefixKindLoopback: {
				validateInputFn:         validateInput,
				validateChildrenExistFn: validateChildrenExist,
				validateNoParentExistFn: validateNoParentExist,
				validateParentExistFn:   validateParentExist,
			},
			ipamv1alpha1.PrefixKindPool: {
				validateInputFn:         validateInput,
				validateChildrenExistFn: validateChildrenExist,
				validateNoParentExistFn: validateNoParentExist,
				validateParentExistFn:   validateParentExist,
			},
			ipamv1alpha1.PrefixKindAggregate: {
				validateInputFn:         validateInput,
				validateChildrenExistFn: validateChildrenExist,
				validateNoParentExistFn: validateNoParentExist,
				validateParentExistFn:   validateParentExist,
			},
		},
	}
}

type ipamPrefixRuntime struct {
	cache   backend.Cache[*table.RIB]
	watcher Watcher
	m       sync.Mutex
	oc      map[ipamv1alpha1.PrefixKind]*PrefixValidatorFunctionConfig
}

func (r *ipamPrefixRuntime) Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// the initializing flag allows to get the rib even when initializing
	// if not set and the rib is initializing an error will be returned
	rib, err := r.cache.Get(alloc.Spec.NetworkInstance, initializing)
	if err != nil {
		return nil, err
	}

	return NewPrefixRuntime(&PrefixRuntimeConfig{
		initializing: initializing,
		alloc:        alloc,
		rib:          rib,
		watcher:      r.watcher,
		fnc:          r.oc[ipamv1alpha1.PrefixKind(*alloc.Spec.Prefix)],
	})

}

func newAllocRuntime(c *RuntimeConfig) Runtimes {
	return &ipamAllocRuntime{
		cache:   c.cache,
		watcher: c.watcher,
		oc: map[ipamv1alpha1.PrefixKind]*AllocValidatorFunctionConfig{
			ipamv1alpha1.PrefixKindNetwork: {
				validateInputFn: validateInput,
			},
			ipamv1alpha1.PrefixKindLoopback: {
				validateInputFn: validateInput,
			},
			ipamv1alpha1.PrefixKindPool: {
				validateInputFn: validateInput,
			},
			ipamv1alpha1.PrefixKindAggregate: {
				validateInputFn: validateInput,
			},
		},
	}
}

type ipamAllocRuntime struct {
	cache   backend.Cache[*table.RIB]
	watcher Watcher
	m       sync.Mutex
	oc      map[ipamv1alpha1.PrefixKind]*AllocValidatorFunctionConfig
}

func (r *ipamAllocRuntime) Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// get rib, returns an error if not yet initialized based on the init flag
	rib, err := r.cache.Get(alloc.Spec.NetworkInstance, initializing)
	if err != nil {
		return nil, err
	}

	return NewAllocRuntime(&AllocRuntimeConfig{
		initializing: initializing,
		alloc:        alloc,
		rib:          rib,
		watcher:      r.watcher,
		fnc:          r.oc[alloc.Spec.Kind],
	})
}
