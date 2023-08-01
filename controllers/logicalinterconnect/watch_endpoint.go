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

package logicalinterconnect

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	topov1alpha1 "github.com/nokia/k8s-ipam/apis/topo/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type endpointEventHandler struct {
	client client.Client
	l      logr.Logger
}

// Create enqueues a request for all ip allocation within the ipam
func (r *endpointEventHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (r *endpointEventHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.ObjectOld, q)
	r.add(ctx, evt.ObjectNew, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (r *endpointEventHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request for all ip allocation within the ipam
func (r *endpointEventHandler) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

func (r *endpointEventHandler) add(ctx context.Context, obj runtime.Object, queue adder) {
	cr, ok := obj.(*invv1alpha1.Endpoint)
	if !ok {
		return
	}
	r.l = log.FromContext(ctx)
	r.l.Info("event", "gvk", fmt.Sprintf("%s.%s", cr.APIVersion, cr.Kind), "name", cr.GetName())

	// if the endpoint was claimed by the logicalInterconnect -> reconcile
	if cr.Status.ClaimRef != nil &&
		cr.Status.ClaimRef.APIVersion == topov1alpha1.GroupVersion.String() &&
		cr.Status.ClaimRef.Kind == topov1alpha1.LogicalInterconnectKind {
		r.l.Info("event requeue logicalinterconnect", "name", cr.Status.ClaimRef.Name)
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: cr.Status.ClaimRef.Namespace,
			Name:      cr.Status.ClaimRef.Name}})

	} else {
		// if the endpoint was not claimed, reconcile logicalinterconnects whose condition is
		// not true -> this allows the logicalinterconnects to reevaluate the endpoints
		opts := []client.ListOption{
			client.InNamespace(cr.Namespace),
		}
		lics := &topov1alpha1.LogicalInterconnectList{}
		if err := r.client.List(ctx, lics, opts...); err != nil {
			r.l.Error(err, "cannot list logicalinterconnects")
			return
		}
		for _, lic := range lics.Items {
			if lic.GetCondition(resourcev1alpha1.ConditionTypeReady).Status == metav1.ConditionFalse {
				r.l.Info("event requeue logicalinterconnect", "name", lic.GetName())
				queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: lic.Namespace,
					Name:      lic.Name}})
			}
		}
	}
}
