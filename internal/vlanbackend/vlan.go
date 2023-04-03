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

	"github.com/go-logr/logr"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Option can be used to manipulate Options.
type Option func(Vlan)

type Vlan interface {
	// Create and initialize the VLAN Database
	Create(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) error
	// Delete the vlan database
	Delete(ctx context.Context, cr *vlanv1alpha1.VLANDatabase)
	// Add a dynamic watch with callback to the ipam rib
	AddWatch(ownerGvkKey, ownerGvk string, fn CallbackFn)
	// Delete a dynamic watch with callback to the ipam rib
	DeleteWatch(ownerGvkKey, ownerGvk string)
	//GetAllocatedVLAN returns the requested allocated vlan if founf
	GetAllocatedVLAN(ctx context.Context, cr *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error)
	// AllocateVLAN allocates a vlan
	AllocateVLAN(ctx context.Context, cr *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error)
	// DeAllocateVLAN deallocates the allocation based on owner selection. No errors are returned if no allocation was found
	DeAllocateVLAN(ctx context.Context, cr *vlanv1alpha1.VLANAllocation) error
	// GetVlans
	GetVlans(cr *vlanv1alpha1.VLANDatabase) db.DB[uint16]
}

func New(c client.Client, opts ...Option) Vlan {
	db := newDB()

	return &be{
		db: db,
	}
}

type be struct {
	c       client.Client
	watcher Watcher
	db      database
	// to add backend
	// tbd if we need runtimes here
	l logr.Logger
}

func (r *be) AddWatch(ownerGvkKey, ownerGvk string, fn CallbackFn) {
	r.watcher.addWatch(ownerGvkKey, ownerGvk, fn)
}
func (r *be) DeleteWatch(ownerGvkKey, ownerGvk string) {
	r.watcher.deleteWatch(ownerGvkKey, ownerGvk)
}

// Create the DB and or restore the DB
func (r *be) Create(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) error {
	id := corev1.ObjectReference{Kind: string(cr.Spec.VLANDBKind), Name: cr.GetName(), Namespace: cr.GetNamespace()}
	r.l = log.FromContext(ctx).WithValues("niRef", id)

	r.l.Info("db create instance start", "isInitialized", r.db.isInitialized(corev1.ObjectReference{Name: cr.GetName(), Namespace: cr.GetNamespace()}))
	// if the DB is not initialaized initialized
	// this happens upon initialization or ipam restart
	r.db.create(id)
	if !r.db.isInitialized(id) {
		if err := r.backend.Restore(ctx, cr); err != nil {
			r.l.Error(err, "backend restore error")
		}

		r.l.Info("db create instance finished")
		return r.db.setInitialized(id)
	}
	r.l.Info("db create instance already initialized")
	return nil
}

// Delete the db instance
func (r *be) Delete(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) {
	id := corev1.ObjectReference{Kind: string(cr.Spec.VLANDBKind), Name: cr.GetName(), Namespace: cr.GetNamespace()}
	r.l = log.FromContext(ctx).WithValues("niRef", id)

	r.l.Info("db delete instance start")
	r.db.delete(id)

	// delete the data from the backend
	if err := r.backend.Delete(ctx, cr); err != nil {
		r.l.Error(err, "backend delete error")
	}

	r.l.Info("db delete instance finished")
}

// GetAllocatedPrefix return the allocated prefic if found
func (r *be) GetAllocatedVLAN(ctx context.Context, cr *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", cr.GetName())
	r.l.Info("get allocated vlan", "selectors", cr.GetSelectorLabels())

	// get the runtime based the following parameters
	// prefixkind
	// hasprefix -> if prefix parsing is nok we return an error
	// networkinstance -> if not initialized we get an error
	// initialized with alloc, rib and prefix if present
	op, err := r.runtimes.Get(cr, false)
	if err != nil {
		return nil, err
	}
	allocatedVLAN, err := op.Get(ctx)
	if err != nil {
		return nil, err
	}
	r.l.Info("get allocated vlan done", "allocatedVLAN", allocatedVLAN)
	return allocatedVLAN, nil

}

func (r *be) AllocateVLAN(ctx context.Context, cr *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error) {
}

// DeAllocateVLAN deallocates the allocation based on owner selection. No errors are returned if no allocation was found
func (r *be) DeAllocateVLAN(ctx context.Context, cr *vlanv1alpha1.VLANAllocation) error {}

// GetVlans
func (r *be) GetVlans(cr *vlanv1alpha1.VLANDatabase) db.DB[uint16] {}
