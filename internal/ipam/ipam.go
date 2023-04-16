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

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func New(c client.Client) (backend.Backend[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation, table.Routes], error) {
	//ipamRib := newIpamRib()
	cache := backend.NewCache[*table.RIB]()
	watcher := newWatcher()
	runtimes := NewRuntimes(&RuntimeConfig{
		cache:   cache,
		watcher: watcher,
	})

	s, err := newCMStorage(&storageConfig{
		client:   c,
		cache:    cache,
		runtimes: runtimes,
	})
	if err != nil {
		return nil, err
	}

	return &be{
		cache:    cache,
		runtimes: runtimes,
		store:    s,
		watcher:  watcher,
	}, nil
}

type be struct {
	watcher  Watcher
	cache    backend.Cache[*table.RIB]
	runtimes Runtimes
	store    Storage[*ipamv1alpha1.IPAllocation, map[string]labels.Set]

	l logr.Logger
}

func (r *be) AddWatch(ownerGvkKey, ownerGvk string, fn backend.CallbackFn) {
	r.watcher.addWatch(ownerGvkKey, ownerGvk, fn)
}
func (r *be) DeleteWatch(ownerGvkKey, ownerGvk string) {
	r.watcher.deleteWatch(ownerGvkKey, ownerGvk)
}

// Create the cache instance and/or restore the cache instance
func (r *be) Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	cacheID := cr.GetCacheID()
	r.l = log.FromContext(ctx).WithValues("cache id", cacheID)

	r.l.Info("create cache instance start", "isInitialized", r.cache.IsInitialized(cacheID))
	// if the Cache is not initialaized initialized
	// this happens upon initialization or backend restart
	r.cache.Create(cacheID, table.NewRIB())
	if !r.cache.IsInitialized(cacheID) {
		if err := r.store.Get().Restore(ctx, cacheID); err != nil {
			r.l.Error(err, "backend cache restore error")
			return err
		}
		r.l.Info("create cache instance finished")
		return r.cache.SetInitialized(cacheID)
	}
	r.l.Info("create cache instance already initialized")
	return nil
}

// Delete the cache instance
func (r *be) Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	cacheID := cr.GetCacheID()
	r.l = log.FromContext(ctx).WithValues("cache id", cacheID)

	r.l.Info("delete cache instance start")
	r.cache.Delete(cacheID)

	// delete the data from the backend
	if err := r.store.Get().Destroy(ctx, cacheID); err != nil {
		r.l.Error(err, "delete cache instance error")
		return err
	}
	r.l.Info("delete cache instance finished")
	return nil
}

func (r *be) List(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) (table.Routes, error) {
	cacheID := cr.GetCacheID()
	r.l = log.FromContext(ctx).WithValues("cache id", cacheID)

	rib, err := r.cache.Get(cacheID, false)
	if err != nil {
		r.l.Error(err, "cannpt get cache instance")
		return []table.Route{}, err
	}
	return rib.GetTable(), nil
}

// List entries in the cache instance
func (r *be) Get(ctx context.Context, a *ipamv1alpha1.IPAllocation) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("get allocated entry", "selectors", a.GetSelectorLabels())

	// get the runtime based the following parameters
	// prefixkind
	// hasprefix -> if prefix parsing is nok we return an error
	// networkinstance -> if not initialized we get an error
	// initialized with alloc, rib and prefix if present
	op, err := r.runtimes.Get(a, false)
	if err != nil {
		return nil, err
	}
	allocatedPrefix, err := op.Get(ctx)
	if err != nil {
		return nil, err
	}
	r.l.Info("get allocated entry done", "allocatedPrefix", allocatedPrefix)
	return allocatedPrefix, nil

}

// AllocateIPPrefix allocates the prefix
func (r *be) Allocate(ctx context.Context, a *ipamv1alpha1.IPAllocation) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("allocate entry", "prefix", a.GetPrefix())

	// get the runtime based the following parameters
	// prefixkind
	// hasprefix -> if prefix parsing is nok we return an error
	// networkinstance -> if not initialized we get an error
	// initialized with alloc, rib and prefix if present
	op, err := r.runtimes.Get(a, false)
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
	//return updatedAlloc, r.updateConfigMap(ctx, alloc)
	return updatedAlloc, r.store.Get().SaveAll(ctx, a.GetNetworkInstance())
}

func (r *be) DeAllocate(ctx context.Context, a *ipamv1alpha1.IPAllocation) error {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())

	// get the runtime based the following parameters
	// prefixkind
	// hasprefix -> if prefix parsing is nok we return an error
	// networkinstance -> if not initialized we get an error
	// initialized with alloc, rib and prefix if present
	rt, err := r.runtimes.Get(a, false)
	if err != nil {
		r.l.Error(err, "cannot get runtime")
		return err
	}
	// we trust the create prefix since it was already allocated
	if err := rt.Delete(ctx); err != nil {
		r.l.Error(err, "cannot deallocate prefix")
		return err
	}
	return r.store.Get().SaveAll(ctx, a.GetNetworkInstance())
}
