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

package backend

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

// newCacheContext holds the cache instance context
// with a status to indicate if it is initialized or not
// initialized false: means it is NOT initialized, 
// initialized true means it is initialized
func newCacheContext[T1 any](i T1) *cacheContext[T1] {
	return &cacheContext[T1]{
		initialized: false,
		instance:    i, 
	}
}

type cacheContext[T1 any] struct {
	initialized bool
	instance    T1
}

func (r *cacheContext[T1]) Initialized() {
	r.initialized = true
}

func (r *cacheContext[T1]) IsInitialized() bool {
	return r.initialized
}

type Cache[T1 any] interface {
	IsInitialized(corev1.ObjectReference) bool
	SetInitialized(corev1.ObjectReference) error
	Get(corev1.ObjectReference, bool) (T1, error)
	Create(corev1.ObjectReference, T1)
	Delete(corev1.ObjectReference)
}

func NewCache[T1 any]() Cache[T1] {
	return &caches[T1]{
		db: map[corev1.ObjectReference]*cacheContext[T1]{},
	}
}

type caches[T1 any] struct {
	m  sync.RWMutex
	db map[corev1.ObjectReference]*cacheContext[T1]
}

func (r *caches[T1]) Create(id corev1.ObjectReference, i T1) {
	r.m.Lock()
	defer r.m.Unlock()
	_, ok := r.db[id]
	if !ok {
		r.db[id] = newCacheContext(i)
	}
}

func (r *caches[T1]) Delete(id corev1.ObjectReference) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.db, id)
}

// init initializes the db
// return true -> isInitialized
// return false -> not initialized
func (r *caches[T1]) IsInitialized(id corev1.ObjectReference) bool {
	r.m.Lock()
	defer r.m.Unlock()
	dbCtx, ok := r.db[id]
	if !ok {
		return false
	}
	return dbCtx.IsInitialized()
}

// initialized sets the status in the ribCtxt to initialized
func (r *caches[T1]) SetInitialized(id corev1.ObjectReference) error {
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
func (r *caches[T1]) Get(id corev1.ObjectReference, ignoreInitializing bool) (T1, error) {
	r.m.RLock()
	defer r.m.RUnlock()
	i, ok := r.db[id]
	if !ok {
		return i.instance, fmt.Errorf("db not initialized: %v", id)
	}
	if !ignoreInitializing && !i.IsInitialized() {
		return i.instance, fmt.Errorf("db is initializing: %v", id)
	}
	return i.instance, nil
}
