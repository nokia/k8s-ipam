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

package ipclaim

import (
	"context"
	"strings"
	"time"

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
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlconfig"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/resource"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	controllers.Register("ipclaim", &reconciler{})
}

const (
	finalizer = "ipam.nephio.org/finalizer"
	// errors
	errGetCr        = "cannot get cr"
	errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=ipam.resource.nephio.org,resources=ipclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.resource.nephio.org,resources=ipclaims/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.resource.nephio.org,resources=ipclaims/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=networkinstances,verbs=get;list;watch

// Setup sets up the controller with the Manager.
func (r *reconciler) Setup(ctx context.Context, mgr ctrl.Manager, cfg *ctrlconfig.ControllerConfig) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {
	// register scheme
	if err := ipamv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	// initialize reconciler
	r.Client = mgr.GetClient()
	r.ClientProxy = cfg.IpamClientProxy
	r.pollInterval = cfg.Poll
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)

	ge := make(chan event.GenericEvent)

	return map[schema.GroupVersionKind]chan event.GenericEvent{ipamv1alpha1.IPClaimGroupVersionKind: ge},
		ctrl.NewControllerManagedBy(mgr).
			For(&ipamv1alpha1.IPClaim{}).
			WatchesRawSource(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
			//Watches(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
			Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	client.Client
	ClientProxy  clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim]
	pollInterval time.Duration
	finalizer    *resource.APIFinalizer

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &ipamv1alpha1.IPClaim{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, "cannot get resource")
			return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get resource")
		}
		return reconcile.Result{}, nil
	}

	if meta.WasDeleted(cr) {
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

	// check the network instance existance, to ensure we update the condition in the cr
	// when a network instance get deleted
	idxName := types.NamespacedName{
		Namespace: cr.GetCacheID().Namespace,
		Name:      cr.GetCacheID().Name,
	}
	idx := &ipamv1alpha1.NetworkInstance{}
	if err := r.Get(ctx, idxName, idx); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		r.l.Info("cannot claim prefix, index not found")
		cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Failed("index not found"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check the network instance existance, to ensure we update the condition in the cr
	// when a network instance get deleted
	if meta.WasDeleted(idx) {
		r.l.Info("cannot claim prefix, index not ready")
		cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Failed("index not ready"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// The spec got changed we check the existing prefix against the status
	// if there is a difference, we need to delete the prefix
	// w/o the prefix in the spec
	specPrefix := cr.Spec.Prefix
	if cr.Status.Prefix != nil && cr.Spec.Prefix != nil &&
		cr.Status.Prefix != cr.Spec.Prefix {
		// we set the prefix to "", to ensure the delete claim works
		cr.Spec.Prefix = nil
		if err := r.ClientProxy.DeleteClaim(ctx, cr, nil); err != nil {
			if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
				r.l.Error(err, "cannot delete resource")
				cr.SetConditions(resourcev1alpha1.ReconcileError(err), resourcev1alpha1.Unknown())
				return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
	}
	cr.Spec.Prefix = specPrefix

	claimResp, err := r.ClientProxy.Claim(ctx, cr, nil)
	if err != nil {
		r.l.Info("cannot claim resource", "err", err)

		// TODO -> Depending on the error we should clear the prefix
		// e.g. when the ni instance is not yet available we should not clear the error
		cr.Status.Gateway = nil
		cr.Status.Prefix = nil
		cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	// if the prefix is claimed in the spec, we need to ensure we get the same claim
	if cr.Spec.Prefix != nil {
		if claimResp.Status.Prefix == nil || *claimResp.Status.Prefix != *cr.Spec.Prefix {
			// we got a different prefix than requested
			r.l.Error(err, "resource claim failed", "requested", cr.Spec.Prefix, "claim Resp", claimResp.Status)
			cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Unknown())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
	}
	cr.Status.Gateway = claimResp.Status.Gateway
	cr.Status.Prefix = claimResp.Status.Prefix
	r.l.Info("Successfully reconciled resource", "claimResp", claimResp.Status)
	cr.SetConditions(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}
