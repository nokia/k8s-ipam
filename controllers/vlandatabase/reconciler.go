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

package vlandatabase

import (
	"context"
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
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlrconfig"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/resource"
	"github.com/pkg/errors"
)

func init() {
	controllers.Register("vlandatabase", &reconciler{})
}

const (
	finalizer = "vlan.nephio.org/finalizer"
	// errors
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=ipam.nephio.org,resources=vlandatabases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=vlandatabases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=vlandatabases/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) Setup(ctx context.Context, mgr ctrl.Manager, cfg *ctrlrconfig.ControllerConfig) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {
	// register scheme
	if err := vlanv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	// initialize reconciler
	r.Client = mgr.GetClient()
	r.ClientProxy = cfg.VlanClientProxy
	r.pollInterval = cfg.Poll
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)

	ge := make(chan event.GenericEvent)

	return map[schema.GroupVersionKind]chan event.GenericEvent{vlanv1alpha1.VLANDatabaseGroupVersionKind: ge},
		ctrl.NewControllerManagedBy(mgr).
			For(&vlanv1alpha1.VLANDatabase{}).
			WatchesRawSource(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
			Complete(r)
}

// reconciler reconciles a NetworkInstance object
type reconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	ClientProxy  clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation]
	pollInterval time.Duration
	finalizer    *resource.APIFinalizer

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &vlanv1alpha1.VLANDatabase{}
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
		if err := r.ClientProxy.DeleteIndex(ctx, cr); err != nil {
			r.l.Error(err, "cannot delete index")
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
	if err := r.ClientProxy.CreateIndex(ctx, cr); err != nil {
		r.l.Error(err, "cannot initialize index")
		cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Failed(err.Error()))
		return ctrl.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// Update the status of the CR and end the reconciliation loop
	cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}
