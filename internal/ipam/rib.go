package ipam

import (
	"fmt"
	"sync"

	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
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
	isInitialized(niRef ipamv1alpha1.NetworkInstanceReference) bool
	setInitialized(niRef ipamv1alpha1.NetworkInstanceReference) error
	getRIB(niRef ipamv1alpha1.NetworkInstanceReference, initializing bool) (*table.RIB, error)
	create(niRef ipamv1alpha1.NetworkInstanceReference)
	delete(niRef ipamv1alpha1.NetworkInstanceReference)
}

func newIpamRib() ipamRib {
	return &ipamrib{
		r: map[ipamv1alpha1.NetworkInstanceReference]*ribContext{},
	}
}

type ipamrib struct {
	m sync.RWMutex
	r map[ipamv1alpha1.NetworkInstanceReference]*ribContext
}

func (r *ipamrib) create(niRef ipamv1alpha1.NetworkInstanceReference) {
	r.m.Lock()
	defer r.m.Unlock()
	_, ok := r.r[niRef]
	if !ok {
		r.r[niRef] = newRibContext()
	}
}

func (r *ipamrib) delete(niRef ipamv1alpha1.NetworkInstanceReference) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.r, niRef)
}

// init initializes the ipamrib
// return true -> isInitialized
// return false -> not initialized
func (r *ipamrib) isInitialized(niRef ipamv1alpha1.NetworkInstanceReference) bool {
	r.m.Lock()
	defer r.m.Unlock()
	ribCtx, ok := r.r[niRef]
	if !ok {
		return false
	}
	return ribCtx.IsInitialized()
}

// initialized sets the status in the ribCtxt to initialized
func (r *ipamrib) setInitialized(niRef ipamv1alpha1.NetworkInstanceReference) error {
	r.m.Lock()
	defer r.m.Unlock()
	ribCtx, ok := r.r[niRef]
	if !ok {
		return fmt.Errorf("network instance not initialized: %v", niRef)
	}
	ribCtx.Initialized()
	return nil
}

// getRIB returns the RIB
// you can ignore the fact the rib is initialized or not using the init flag
func (r *ipamrib) getRIB(niRef ipamv1alpha1.NetworkInstanceReference, ignoreInitializing bool) (*table.RIB, error) {
	r.m.Lock()
	defer r.m.Unlock()
	ii, ok := r.r[niRef]
	if !ok {
		return nil, fmt.Errorf("network instance not initialized: %v", niRef)
	}
	if !ignoreInitializing && !ii.IsInitialized() {
		return nil, fmt.Errorf("network instance is initializing: %v", niRef)
	}
	return ii.rib, nil
}
