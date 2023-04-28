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

package ipamnetworkinstance

import (
	"context"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/resource"
	"github.com/nokia/k8s-ipam/internal/shared"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/pkg/errors"
)

const (
	finalizer = "ipam.nephio.org/finalizer"
	// errors
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"
)

//+kubebuilder:rbac:groups=ipam.nephio.org,resources=networkinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=networkinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=networkinstances/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=networkinstances,verbs=get;list;watch

// SetupWithManager sets up the controller with the Manager.
func Setup(mgr ctrl.Manager, options *shared.Options) (schema.GroupVersionKind, chan event.GenericEvent, error) {
	ge := make(chan event.GenericEvent)
	r := &reconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		IpamClientProxy: options.IpamClientProxy,
		pollInterval:    options.Poll,
		finalizer:       resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
	}

	return ipamv1alpha1.NetworkInstanceGroupVersionKind, ge,
		ctrl.NewControllerManagedBy(mgr).
			For(&ipamv1alpha1.NetworkInstance{}).
			Watches(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
			Complete(r)
}

// reconciler reconciles a NetworkInstance object
type reconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	IpamClientProxy clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]
	pollInterval    time.Duration
	finalizer       *resource.APIFinalizer

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &ipamv1alpha1.NetworkInstance{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, "cannot get resource")
			return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get resource")
		}
		return ctrl.Result{}, nil
	}

	if meta.WasDeleted(cr) {

		// When the network instance is deleted we can remove the network instance entry
		// from th IPAM table
		if err := r.IpamClientProxy.DeleteIndex(ctx, cr); err != nil {
			r.l.Error(err, "cannot delete networkInstance")
			cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Unknown())
			return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Unknown())
			return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		r.l.Info("Successfully deleted resource")
		return ctrl.Result{Requeue: false}, nil
	}

	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		r.l.Error(err, "cannot add finalizer")
		cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Unknown())
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// create and initialize the IPAM with the network instance if it does not exist
	// the prefixes that are within the spec of the network-instance need to be allocated first
	// since they serve as an aggregate
	if err := r.IpamClientProxy.CreateIndex(ctx, cr); err != nil {
		r.l.Error(err, "cannot initialize ipam")
		cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// change handling for prefixes
	for _, allocatedPrefix := range cr.Status.Prefixes {
		found := false
		for _, prefix := range cr.Spec.Prefixes {
			if allocatedPrefix.Prefix == prefix.Prefix {
				found = true
				break
			}
		}
		if !found {
			// the prefix was deleted from the network instance, so we need to delete it
			if err := r.IpamClientProxy.DeAllocate(ctx, cr, allocatedPrefix); err != nil {
				if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
					r.l.Error(err, "cannot delete resource")
					cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Failed(err.Error()))
					return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
				}
			}
		}
	}

	// if prefixes are provided from the network instance we treat them as
	// aggregate prefixes.
	for _, prefix := range cr.Spec.Prefixes {
		allocResp, err := r.IpamClientProxy.Allocate(ctx, cr, prefix)
		if err != nil {
			r.l.Info("cannot allocate prefix", "err", err)
			cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Failed(err.Error()))
			return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		if allocResp.Status.Prefix == nil || *allocResp.Status.Prefix != prefix.Prefix {
			//we got a different prefix than requested
			r.l.Error(err, "prefix allocation failed", "requested", prefix.Prefix, "allocated", allocResp.Status.Prefix)
			cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Unknown())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
	}

	cr.Status.Prefixes = cr.Spec.Prefixes

	// Update the status of the CR and end the reconciliation loop
	cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}
