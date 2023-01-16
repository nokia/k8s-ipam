package ipam

import (
	"context"
	"sync"

	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
)

type Runtimes interface {
	Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error)
	//GetPrefixRuntime() runtime
	//GetAllocRuntime() runtime
}

type RuntimeConfig struct {
	ipamRib ipamRib
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
	if alloc.GetPrefix() == "" {
		return r.allocRuntime.Get(alloc, initializing)
	}
	return r.prefixRuntime.Get(alloc, initializing)
}

/*
func (r *runtimes) GetPrefixRuntime() runtime {
	return r.prefixRuntime
}

func (r *runtimes) GetAllocRuntime() runtime {
	return r.allocRuntime
)
*/

type runtime interface {
	Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error)
}

type Runtime interface {
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
		ipamRib: c.ipamRib,
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
	ipamRib ipamRib
	watcher Watcher
	m       sync.Mutex
	oc      map[ipamv1alpha1.PrefixKind]*PrefixValidatorFunctionConfig
}

func (r *ipamPrefixRuntime) Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// the initializing flag allows to get the rib even when initializing
	// if not set and the rib is initializing an error will be returned
	rib, err := r.ipamRib.getRIB(alloc.GetNetworkInstance(), initializing)
	if err != nil {
		return nil, err
	}

	return NewPrefixRuntime(&PrefixRuntimeConfig{
		initializing: initializing,
		alloc:        alloc,
		rib:          rib,
		watcher:      r.watcher,
		fnc:          r.oc[alloc.GetPrefixKind()],
	})

}

func newAllocRuntime(c *RuntimeConfig) Runtimes {
	return &ipamAllocRuntime{
		ipamRib: c.ipamRib,
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
	ipamRib ipamRib
	watcher Watcher
	m       sync.Mutex
	oc      map[ipamv1alpha1.PrefixKind]*AllocValidatorFunctionConfig
}

func (r *ipamAllocRuntime) Get(alloc *ipamv1alpha1.IPAllocation, initializing bool) (Runtime, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// get rib, returns an error if not yet initialized based on the init flag
	rib, err := r.ipamRib.getRIB(alloc.GetNetworkInstance(), initializing)
	if err != nil {
		return nil, err
	}

	return NewAllocOperator(&AllocRuntimeConfig{
		initializing: initializing,
		alloc:        alloc,
		rib:          rib,
		watcher:      r.watcher,
		fnc:          r.oc[alloc.GetPrefixKind()],
	})

}
