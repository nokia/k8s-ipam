/*
Copyright 2023 The Nephio Authors.

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
package vlanvlan

import (
	"context"

	"github.com/go-logr/logr"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type adder interface {
	Add(item interface{})
}

type EnqueueRequestForAllVlanDatabases struct {
	client client.Client
	l      logr.Logger
	ctx    context.Context
}

// Create enqueues a request for create
func (e *EnqueueRequestForAllVlanDatabases) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for update
func (e *EnqueueRequestForAllVlanDatabases) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.ObjectOld, q)
	e.add(evt.ObjectNew, q)
}

// Create enqueues a request for delete
func (e *EnqueueRequestForAllVlanDatabases) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for generic event
func (e *EnqueueRequestForAllVlanDatabases) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

func (e *EnqueueRequestForAllVlanDatabases) add(obj runtime.Object, queue adder) {
	idx, ok := obj.(*vlanv1alpha1.VLANDatabase)
	if !ok {
		return
	}
	e.l = log.FromContext(e.ctx)
	e.l.Info("event", "kind", obj.GetObjectKind(), "name", idx.GetName())

	d := &vlanv1alpha1.VLANList{}
	if err := e.client.List(e.ctx, d); err != nil {
		return
	}

	for _, r := range d.Items {
		// only enqueue if the index matches
		if idx.GetCacheID().Name == r.GetCacheID().Name && idx.GetCacheID().Namespace == r.GetCacheID().Namespace {
			e.l.Info("event requeue allocation", "name", r.GetName())
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: r.GetNamespace(),
				Name:      r.GetName()}})
		}
	}
}
