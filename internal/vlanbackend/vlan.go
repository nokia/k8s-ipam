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
	"context"
	"fmt"

	"github.com/go-logr/logr"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/internal/db/vlandb"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func New(c client.Client) (backend.Backend[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation, db.Entries[uint16]], error) {

	ca := backend.NewCache[db.DB[uint16]]()

	s, err := newCMStorage(&storageConfig{
		client: c,
		cache:  ca,
	})
	if err != nil {
		return nil, err
	}

	return &be{
		cache: ca,
		store: s,
	}, nil
}

type be struct {
	watcher Watcher
	cache   backend.Cache[db.DB[uint16]]
	store   Storage[*vlanv1alpha1.VLANAllocation, map[string]labels.Set]
	l       logr.Logger
}

func (r *be) AddWatch(ownerGvkKey, ownerGvk string, fn backend.CallbackFn) {
	r.watcher.addWatch(ownerGvkKey, ownerGvk, fn)
}
func (r *be) DeleteWatch(ownerGvkKey, ownerGvk string) {
	r.watcher.deleteWatch(ownerGvkKey, ownerGvk)
}

// Create the cache instance and/or restore the cache instance
func (r *be) Create(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) error {
	cacheID := cr.GetCacheID()
	r.l = log.FromContext(ctx).WithValues("cache id", cacheID)

	r.l.Info("create cache instance start", "isInitialized", r.cache.IsInitialized(cacheID))
	// if the Cache is not initialaized initialized
	// this happens upon initialization or backend restart
	r.cache.Create(cacheID, vlandb.New())
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
func (r *be) Delete(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) error {
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

// List entries in the db instance
func (r *be) List(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) (db.Entries[uint16], error) {
	cacheID := cr.GetCacheID()
	r.l = log.FromContext(ctx).WithValues("cache id", cacheID)

	d, err := r.cache.Get(cacheID, false)
	if err != nil {
		r.l.Error(err, "cannpt get cache instance")
		return db.Entries[uint16]{}, nil
	}
	return d.GetAll(), err
}

// Gwt return the allocated entry if found
func (r *be) Get(ctx context.Context, a *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("get allocated entry", "selectors", a.GetSelectorLabels())

	al, err := r.newApplogic(a, false)
	if err != nil {
		return nil, err
	}
	a, err = al.Get(ctx, a)
	if err != nil {
		return nil, err
	}

	r.l.Info("get allocated entry done", "allocatedVLAN", a.Status)
	return a, nil
}

func (r *be) Allocate(ctx context.Context, a *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("allocate")

	al, err := r.newApplogic(a, false)
	if err != nil {
		return nil, err
	}
	msg, err := al.Validate(ctx, a)
	if err != nil {
		return nil, err
	}
	if msg != "" {
		r.l.Error(fmt.Errorf("%s", msg), "validation failed")
		return nil, fmt.Errorf("validation failed: %s", msg)
	}
	a, err = al.Apply(ctx, a)
	if err != nil {
		return nil, err
	}

	r.l.Info("allocate  done", "updatedAlloc", a)
	return a, nil
	//r.store.Get().SaveAll(ctx, *a.Spec.VLANDatabases[0])
}

// DeAllocateVLAN deallocates the allocation based on owner selection. No errors are returned if no allocation was found
func (r *be) DeAllocate(ctx context.Context, a *vlanv1alpha1.VLANAllocation) error {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("deallocate")

	al, err := r.newApplogic(a, false)
	if err != nil {
		return err
	}
	if err := al.Delete(ctx, a); err != nil {
		r.l.Error(err, "cannot deallocate prefix")
		return err
	}

	return r.store.Get().SaveAll(ctx, *a.Spec.VLANDatabases[0])
}
