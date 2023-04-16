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
package allocation

import (
	"context"

	//ndddvrv1 "github.com/yndd/ndd-core/apis/dvr/v1"
	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
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

type EnqueueRequestForAllNetworkInstances struct {
	client client.Client
	l      logr.Logger
	ctx    context.Context
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllNetworkInstances) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllNetworkInstances) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.ObjectOld, q)
	e.add(evt.ObjectNew, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllNetworkInstances) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (e *EnqueueRequestForAllNetworkInstances) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

func (e *EnqueueRequestForAllNetworkInstances) add(obj runtime.Object, queue adder) {
	ni, ok := obj.(*ipamv1alpha1.NetworkInstance)
	if !ok {
		return
	}
	e.l = log.FromContext(e.ctx)
	e.l.Info("event", "kind", obj.GetObjectKind(), "name", ni.GetName())

	d := &ipamv1alpha1.IPAllocationList{}
	if err := e.client.List(e.ctx, d); err != nil {
		return
	}

	for _, alloc := range d.Items {
		// only enqueue if the network-instance matches
		allocNiNamespace := "default"
		if alloc.GetNetworkInstance().Namespace != "" {
			allocNiNamespace = alloc.GetNetworkInstance().Namespace
		}
		if ni.GetName() == alloc.GetNetworkInstance().Name && ni.GetNamespace() == allocNiNamespace {
			e.l.Info("event requeue allocation", "name", alloc.GetName())
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: alloc.GetNamespace(),
				Name:      alloc.GetName()}})
		}
	}
}
