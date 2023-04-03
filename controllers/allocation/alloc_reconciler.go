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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/resource"
	"github.com/nokia/k8s-ipam/internal/shared"
	"github.com/nokia/k8s-ipam/pkg/ipam/clientproxy"
	"github.com/pkg/errors"
)

const (
	finalizer = "ipam.nephio.org/finalizer"
	// errors
	errGetCr        = "cannot get cr"
	errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=ipam.nephio.org,resources=ipallocations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=ipallocations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=ipallocations/finalizers,verbs=update
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

	/*
		niHandler := &EnqueueRequestForAllNetworkInstances{
			client: mgr.GetClient(),
			ctx:    context.Background(),
		}
	*/

	return ipamv1alpha1.IPAllocationGroupVersionKind, ge, ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPAllocation{}).
		//Watches(&source.Kind{Type: &ipamv1alpha1.NetworkInstance{}}, niHandler).
		Watches(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	IpamClientProxy clientproxy.Proxy
	pollInterval    time.Duration
	finalizer       *resource.APIFinalizer

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &ipamv1alpha1.IPAllocation{}
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
		if cr.GetCondition(allocv1alpha1.ConditionKindReady).Status == corev1.ConditionTrue {
			if err := r.IpamClientProxy.DeAllocateIPPrefix(ctx, cr, nil); err != nil {
				if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
					r.l.Error(err, "cannot delete resource")
					cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Unknown())
					return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
				}
			}
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Unknown())
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
		cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check if the network instance exists in the allocation request
	//niName, ok := cr.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey]
	//if !ok {
	//	r.l.Info("cannot allocate prefix, network-intance not found in cr")
	//	cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("network-instance not found in cr"))
	//	return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	//}

	// check the network instance existance, to ensure we update the condition in the cr
	// when a network instance get deleted
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: cr.Spec.NetworkInstance.Namespace,
		Name:      cr.Spec.NetworkInstance.Name,
	}, ni); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		r.l.Info("cannot allocate prefix, network-intance not found")
		cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Failed("network-instance not found"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check the network instance existance, to ensure we update the condition in the cr
	// when a network instance get deleted
	if meta.WasDeleted(ni) {
		r.l.Info("cannot allocate prefix, network-intance not ready")
		cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Failed("network-instance not ready"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// for prefixKind network validate if the label exists
	/*
		if cr.Spec.PrefixKind == ipamv1alpha1.PrefixKindNetwork {
			_, ok := cr.Spec.Selector.MatchLabels[ipamv1alpha1.NephioSubnetNameKey]
			if !ok {
				r.l.Info("cannot allocate prefix, matchLabels must contain a network key")
				cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("cannot allocate prefix, matchLabels must contain a network key"))
				return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
	*/

	// set origin to label allocation
	/*
		if len(cr.Labels) == 0 {
			cr.Labels = map[string]string{}
		}
		cr.Labels[ipamv1alpha1.NephioOriginKey] = string(ipamv1alpha1.OriginIPAllocation)
	*/
	// The spec got changed we check the existing prefix against the status
	// if there is a difference, we need to delete the prefix
	// w/o the prefix in the spec
	specPrefix := cr.Spec.Prefix
	if cr.Status.AllocatedPrefix != "" && cr.Spec.Prefix != "" &&
		cr.Status.AllocatedPrefix != cr.Spec.Prefix {
		// we set the prefix to "", to ensure the deallocation works
		cr.Spec.Prefix = ""
		if err := r.IpamClientProxy.DeAllocateIPPrefix(ctx, cr, nil); err != nil {
			if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
				r.l.Error(err, "cannot delete resource")
				cr.SetConditions(allocv1alpha1.ReconcileError(err), allocv1alpha1.Unknown())
				return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
	}
	cr.Spec.Prefix = specPrefix

	allocResp, err := r.IpamClientProxy.AllocateIPPrefix(ctx, cr, nil)
	if err != nil {
		r.l.Info("cannot allocate prefix", "err", err)

		// TODO -> Depending on the error we should clear the prefix
		// e.g. when the ni instance is not yet available we should not clear the error
		cr.Status.Gateway = ""
		cr.Status.AllocatedPrefix = ""
		cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Failed(err.Error()))
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	// if the prefix is allocated in the spec, we need to ensure we get the same allocation
	if cr.Spec.Prefix != "" {
		if allocResp.Status.AllocatedPrefix != cr.Spec.Prefix {
			// we got a different prefix than requested
			r.l.Error(err, "prefix allocation failed", "requested", cr.Spec.Prefix, "alloc Resp", allocResp.Status)
			cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Unknown())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
	}
	cr.Status.Gateway = allocResp.Status.Gateway
	cr.Status.AllocatedPrefix = allocResp.Status.AllocatedPrefix
	r.l.Info("Successfully reconciled resource", "allocResp", allocResp.Status)
	cr.SetConditions(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}
