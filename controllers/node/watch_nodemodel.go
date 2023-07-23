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

package node

import (
	"context"

	"github.com/go-logr/logr"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type nodeModelEventHandler struct {
	client client.Client
	l      logr.Logger
}

// Create enqueues a request for all ip allocation within the ipam
func (r *nodeModelEventHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (r *nodeModelEventHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.ObjectOld, q)
	r.add(ctx, evt.ObjectNew, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (r *nodeModelEventHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (r *nodeModelEventHandler) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

func (r *nodeModelEventHandler) add(ctx context.Context, obj runtime.Object, queue adder) {
	cr, ok := obj.(*invv1alpha1.NodeModel)
	if !ok {
		return
	}
	r.l = log.FromContext(ctx)
	r.l.Info("event", "kind", obj.GetObjectKind(), "name", cr.GetName())

	// only select the nodes belonging to this provider
	opts := []client.ListOption{
		client.MatchingLabels{
			invv1alpha1.NephioProviderKey: cr.Spec.Provider,
		},
		client.InNamespace(cr.Namespace),
	}

	nodes := &invv1alpha1.NodeList{}
	if err := r.client.List(ctx, nodes, opts...); err != nil {
		r.l.Error(err, "cannot list nodes")
		return
	}

	for _, node := range nodes.Items {
		// only enqueue if the provider and the networktopology match
		if node.Status.UsedNodeModelRef != nil && (node.Status.UsedNodeModelRef.Name == cr.Name) {
			r.l.Info("event requeue node", "name", node.GetName())
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: node.GetNamespace(),
				Name:      node.GetName()}})
		}
	}
}
