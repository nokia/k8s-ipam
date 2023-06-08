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
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
)

type Runtimes interface {
	Get(claim *ipamv1alpha1.IPClaim, initializing bool) (Runtime, error)
}

type RuntimeConfig struct {
	cache   backend.Cache[*table.RIB]
	watcher Watcher
}

func NewRuntimes(c *RuntimeConfig) Runtimes {
	return &runtimes{
		prefixRuntime:  newPrefixRuntime(c),
		dynamicRuntime: newDynamicRuntime(c),
	}
}

type runtimes struct {
	prefixRuntime  runtime
	dynamicRuntime runtime
}

func (r *runtimes) Get(claim *ipamv1alpha1.IPClaim, initializing bool) (Runtime, error) {
	if claim.Spec.Prefix == nil {
		return r.dynamicRuntime.Get(claim, initializing)
	}
	return r.prefixRuntime.Get(claim, initializing)
}

type runtime interface {
	Get(claim *ipamv1alpha1.IPClaim, initializing bool) (Runtime, error)
}

type Runtime interface {
	Get(ctx context.Context) (*ipamv1alpha1.IPClaim, error)
	Validate(ctx context.Context) (string, error)
	Apply(ctx context.Context) (*ipamv1alpha1.IPClaim, error)
	Delete(ctx context.Context) error
}

type PrefixRuntimeConfig struct {
	initializing bool
	claim        *ipamv1alpha1.IPClaim
	rib          *table.RIB
	fnc          *PrefixValidatorFunctionConfig
	watcher      Watcher
}

type DynamicRuntimeConfig struct {
	initializing bool
	claim        *ipamv1alpha1.IPClaim
	rib          *table.RIB
	fnc          *DynamicValidatorFunctionConfig
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

func (r *ipamPrefixRuntime) Get(claim *ipamv1alpha1.IPClaim, initializing bool) (Runtime, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// the initializing flag allows to get the rib even when initializing
	// if not set and the rib is initializing an error will be returned
	rib, err := r.cache.Get(claim.GetCacheID(), initializing)
	if err != nil {
		return nil, err
	}

	return NewPrefixRuntime(&PrefixRuntimeConfig{
		initializing: initializing,
		claim:        claim,
		rib:          rib,
		watcher:      r.watcher,
		fnc:          r.oc[claim.Spec.Kind],
	})

}

func newDynamicRuntime(c *RuntimeConfig) Runtimes {
	return &ipamDynamicRuntime{
		cache:   c.cache,
		watcher: c.watcher,
		oc: map[ipamv1alpha1.PrefixKind]*DynamicValidatorFunctionConfig{
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

type ipamDynamicRuntime struct {
	cache   backend.Cache[*table.RIB]
	watcher Watcher
	m       sync.Mutex
	oc      map[ipamv1alpha1.PrefixKind]*DynamicValidatorFunctionConfig
}

func (r *ipamDynamicRuntime) Get(claim *ipamv1alpha1.IPClaim, initializing bool) (Runtime, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// get rib, returns an error if not yet initialized based on the init flag
	rib, err := r.cache.Get(claim.GetCacheID(), initializing)
	if err != nil {
		return nil, err
	}

	return NewDynamicRuntime(&DynamicRuntimeConfig{
		initializing: initializing,
		claim:        claim,
		rib:          rib,
		watcher:      r.watcher,
		fnc:          r.oc[claim.Spec.Kind],
	})
}
