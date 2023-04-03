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

package vlanbackend

import (
	"fmt"
	"sync"

	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/internal/db/vlandb"
	corev1 "k8s.io/api/core/v1"
)

// newDBContext holds the db.DB instance context
// with a status to indicate if it is initialized or not
// initialized false: means it is NOT initialized, initialized true means it is initialized
func newDBContext() *dbContext {
	return &dbContext{
		initialized:    false,
		vlanDBInstance: vlandb.New[uint16](), // this makes it specific
	}
}

type dbContext struct {
	initialized    bool
	vlanDBInstance db.DB[uint16]
}

func (r *dbContext) Initialized() {
	r.initialized = true
}

func (r *dbContext) IsInitialized() bool {
	return r.initialized
}

type database interface {
	isInitialized(niRef corev1.ObjectReference) bool
	setInitialized(niRef corev1.ObjectReference) error
	get(niRef corev1.ObjectReference, initializing bool) (db.DB[uint16], error)
	create(niRef corev1.ObjectReference)
	delete(niRef corev1.ObjectReference)
}

func newDB() database {
	return &dbs{
		db: map[corev1.ObjectReference]*dbContext{},
	}
}

type dbs struct {
	m  sync.RWMutex
	db map[corev1.ObjectReference]*dbContext
}

func (r *dbs) create(id corev1.ObjectReference) {
	r.m.Lock()
	defer r.m.Unlock()
	_, ok := r.db[id]
	if !ok {
		r.db[id] = newDBContext()
	}
}

func (r *dbs) delete(id corev1.ObjectReference) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.db, id)
}

// init initializes the db
// return true -> isInitialized
// return false -> not initialized
func (r *dbs) isInitialized(id corev1.ObjectReference) bool {
	r.m.Lock()
	defer r.m.Unlock()
	dbCtx, ok := r.db[id]
	if !ok {
		return false
	}
	return dbCtx.IsInitialized()
}

// initialized sets the status in the ribCtxt to initialized
func (r *dbs) setInitialized(id corev1.ObjectReference) error {
	r.m.Lock()
	defer r.m.Unlock()
	dbCtx, ok := r.db[id]
	if !ok {
		return fmt.Errorf("db not initialized: %v", id)
	}
	dbCtx.Initialized()
	return nil
}

// getRIB returns the RIB
// you can ignore the fact the rib is initialized or not using the init flag
func (r *dbs) get(id corev1.ObjectReference, ignoreInitializing bool) (db.DB[uint16], error) {
	r.m.Lock()
	defer r.m.Unlock()
	ii, ok := r.db[id]
	if !ok {
		return nil, fmt.Errorf("db not initialized: %v", id)
	}
	if !ignoreInitializing && !ii.IsInitialized() {
		return nil, fmt.Errorf("db is initializing: %v", id)
	}
	return ii.vlanDBInstance, nil
}
