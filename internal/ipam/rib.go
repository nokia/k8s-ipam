package ipam

import (
	"fmt"
	"sync"

	"github.com/hansthienpondt/nipam/pkg/table"
	corev1 "k8s.io/api/core/v1"
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
	isInitialized(niRef corev1.ObjectReference) bool
	setInitialized(niRef corev1.ObjectReference) error
	getRIB(niRef corev1.ObjectReference, initializing bool) (*table.RIB, error)
	create(niRef corev1.ObjectReference)
	delete(niRef corev1.ObjectReference)
}

func newIpamRib() ipamRib {
	return &ipamrib{
		r: map[corev1.ObjectReference]*ribContext{},
	}
}

type ipamrib struct {
	m sync.RWMutex
	r map[corev1.ObjectReference]*ribContext
}

func (r *ipamrib) create(niRef corev1.ObjectReference) {
	r.m.Lock()
	defer r.m.Unlock()
	_, ok := r.r[niRef]
	if !ok {
		r.r[niRef] = newRibContext()
	}
}

func (r *ipamrib) delete(niRef corev1.ObjectReference) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.r, niRef)
}

// init initializes the ipamrib
// return true -> isInitialized
// return false -> not initialized
func (r *ipamrib) isInitialized(niRef corev1.ObjectReference) bool {
	r.m.Lock()
	defer r.m.Unlock()
	ribCtx, ok := r.r[niRef]
	if !ok {
		return false
	}
	return ribCtx.IsInitialized()
}

// initialized sets the status in the ribCtxt to initialized
func (r *ipamrib) setInitialized(niRef corev1.ObjectReference) error {
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
func (r *ipamrib) getRIB(niRef corev1.ObjectReference, ignoreInitializing bool) (*table.RIB, error) {
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
