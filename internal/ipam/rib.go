package ipam

import (
	"fmt"
	"sync"

	//"github.com/hansthienpondt/goipam/pkg/table"
	"github.com/hansthienpondt/nipam/pkg/table"
)

// newRibContext holds the rib/patricia tree context
// with a status to indicate if it is initialized or not
// init false: means it is NOT initialized, init true means it is initialized
func newRibContext() *ribContext {
	return &ribContext{
		init: true,
		rib:  table.NewRIB(),
	}
}

type ribContext struct {
	init bool
	rib  *table.RIB
}

func (r *ribContext) InitDone() {
	r.init = false
}

func (r *ribContext) IsInit() bool {
	return r.init
}

type ipamRib interface {
	initializing(niName string) bool
	initDone(niName string) error
	getRIB(niName string, initializing bool) (*table.RIB, error)
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

// init initializes the ipamrib
// return true -> initializing
// return false -> already initialized
func (r *ipamrib) initializing(crName string) bool {
	r.m.Lock()
	defer r.m.Unlock()
	_, ok := r.r[crName]
	if !ok {
		r.r[crName] = newRibContext()
		return true
	}
	return false
}

// initDone sets the status in the ribCtxt to initialized
func (r *ipamrib) initDone(crName string) error {
	r.m.Lock()
	defer r.m.Unlock()
	ribCtx, ok := r.r[crName]
	if !ok {
		return fmt.Errorf("network instance not initialized: %s", crName)
	}
	ribCtx.InitDone()
	return nil
}

// getRIB returns the RIB
// you can ignore the fact the rib is initialized or not using the init flag
func (r *ipamrib) getRIB(niName string, initializing bool) (*table.RIB, error) {
	r.m.Lock()
	defer r.m.Unlock()
	ii, ok := r.r[niName]
	if !ok {
		return nil, fmt.Errorf("network instance not initialized: %s", niName)
	}
	if !initializing && ii.IsInit() {
		return nil, fmt.Errorf("network instance is initializing: %s", niName)
	}
	return ii.rib, nil
}

func (r *ipamrib) delete(niName string) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.r, niName)
}


