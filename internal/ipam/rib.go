package ipam

import (
	"fmt"
	"sync"

	"github.com/hansthienpondt/nipam/pkg/table"
)

// newRibContext holds the rib/patricia tree context
// with a status to indicate if it is initialized or not
// init false: means it is NOT initialized, init true means it is initialized
func newRibContext() *ribContext {
	return &ribContext{
		initialized: false,
		rib:         table.NewRIB(),
	}
}

type ribContext struct {
	initialized bool
	rib         *table.RIB
}

func (r *ribContext) Initialized() {
	r.initialized = true
}

func (r *ribContext) IsInitialized() bool {
	return r.initialized
}

type ipamRib interface {
	isInitialized(niName string) bool
	initialized(niName string) error
	getRIB(niName string, initializing bool) (*table.RIB, error)
	create(niName string)
	delete(niName string)
}

func newIpamRib() ipamRib {
	return &ipamrib{
		r: map[string]*ribContext{},
	}
}

type ipamrib struct {
	m sync.RWMutex
	r map[string]*ribContext
}

func (r *ipamrib) create(niName string) {
	r.m.Lock()
	defer r.m.Unlock()
	_, ok := r.r[niName]
	if !ok {
		r.r[niName] = newRibContext()
	}
}

func (r *ipamrib) delete(niName string) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.r, niName)
}

// init initializes the ipamrib
// return true -> isInitialized
// return false -> not initialized
func (r *ipamrib) isInitialized(niName string) bool {
	r.m.Lock()
	defer r.m.Unlock()
	ribCtx, ok := r.r[niName]
	if !ok {
		return false
	}
	return ribCtx.IsInitialized()
}

// initialized sets the status in the ribCtxt to initialized
func (r *ipamrib) initialized(niName string) error {
	r.m.Lock()
	defer r.m.Unlock()
	ribCtx, ok := r.r[niName]
	if !ok {
		return fmt.Errorf("network instance not initialized: %s", niName)
	}
	ribCtx.Initialized()
	return nil
}

// getRIB returns the RIB
// you can ignore the fact the rib is initialized or not using the init flag
func (r *ipamrib) getRIB(niName string, ignoreInitializing bool) (*table.RIB, error) {
	r.m.Lock()
	defer r.m.Unlock()
	ii, ok := r.r[niName]
	if !ok {
		return nil, fmt.Errorf("network instance not initialized: %s", niName)
	}
	if !ignoreInitializing && !ii.IsInitialized() {
		return nil, fmt.Errorf("network instance is initializing: %s", niName)
	}
	return ii.rib, nil
}
