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
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlconfig"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/resource"
	"github.com/pkg/errors"
)

func init() {
	controllers.Register("vlan", &reconciler{})
}

const (
	finalizer = "vlan.nephio.org/finalizer"
	// error
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=vlan.resource.nephio.org,resources=vlans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vlan.resource.nephio.org,resources=vlans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vlan.resource.nephio.org,resources=vlans/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) Setup(ctx context.Context, mgr ctrl.Manager, cfg *ctrlconfig.ControllerConfig) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {
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

	return map[schema.GroupVersionKind]chan event.GenericEvent{vlanv1alpha1.VLANGroupVersionKind: ge},
		ctrl.NewControllerManagedBy(mgr).
			Named("VLANController").
			For(&vlanv1alpha1.VLAN{}).
			WatchesRawSource(source.Channel(ge, &handler.EnqueueRequestForObject{})).
			//Watches(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
			Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	ClientProxy  clientproxy.Proxy[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim]
	pollInterval time.Duration
	finalizer    *resource.APIFinalizer

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &vlanv1alpha1.VLAN{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, "cannot get resource")
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get resource")
		}
		return reconcile.Result{}, nil
	}

	if meta.WasDeleted(cr) {
		// if the prefix condition is false it means the prefix was not active in the ipam
		// we can delete it w/o deleting it from the IPAM
		if cr.GetCondition(resourcev1alpha1.ConditionTypeReady).Status == metav1.ConditionTrue {
			if err := r.ClientProxy.DeleteClaim(ctx, cr, nil); err != nil {
				if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
					r.l.Error(err, "cannot delete resource")
					cr.SetConditions(resourcev1alpha1.ReconcileError(err), resourcev1alpha1.Unknown())
					return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
				}
			}
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(resourcev1alpha1.ReconcileError(err), resourcev1alpha1.Unknown())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		r.l.Info("Successfully deleted resource")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		r.l.Error(err, "cannot add finalizer")
		cr.SetConditions(resourcev1alpha1.ReconcileError(err), resourcev1alpha1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// this block is here to deal with index deletion
	// we ensure the condition is set to false if the index is deleted
	idxName := types.NamespacedName{
		Namespace: cr.GetCacheID().Namespace,
		Name:      cr.GetCacheID().Name,
	}
	idx := &vlanv1alpha1.VLANIndex{}
	if err := r.Get(ctx, idxName, idx); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		r.l.Info("cannot claim resource, index not found", "idx", idxName)
		cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Failed("index not found"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check deletion timestamp of the network instance
	if meta.WasDeleted(idx) {
		r.l.Info("cannot claim resource, network-intance not ready")
		cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Failed("network-instance not ready"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// The spec got changed we check the existing prefix against the status
	// if there is a difference, we need to delete the prefix
	if cr.Status.VLANID != nil && cr.Spec.VLANID != nil &&
		*cr.Status.VLANID != *cr.Spec.VLANID {
		r.l.Info("delete claim", "spec VLANID", cr.Spec.VLANID, "status VLANID", cr.Status.VLANID)
		if err := r.ClientProxy.DeleteClaim(ctx, cr, nil); err != nil {
			if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") || !strings.Contains(err.Error(), "initalizing") {
				r.l.Error(err, "cannot delete resource")
				cr.SetConditions(resourcev1alpha1.ReconcileError(err), resourcev1alpha1.Unknown())
				return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
	}
	// TODO VLAN Range

	claimResp, err := r.ClientProxy.Claim(ctx, cr, nil)
	if err != nil {
		r.l.Error(err, "cannot claim resource")
		cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	if *claimResp.Status.VLANID != *cr.Spec.VLANID {
		//we got a different prefix than requested
		r.l.Error(err, "prefix claim failed", "requested", cr.Spec.VLANID, "claimed", claimResp.Status.VLANID)
		cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Unknown())
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	r.l.Info("Successfully reconciled resource")
	cr.Status.VLANID = cr.Spec.VLANID
	cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}
