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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/ipam"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/resource"
	"github.com/nokia/k8s-ipam/internal/shared"
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
func Setup(mgr ctrl.Manager, options *shared.Options) error {
	r := &reconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Ipam:         options.Ipam,
		pollInterval: options.Poll,
		finalizer:    resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
	}

	/*
		niHandler := &EnqueueRequestForAllNetworkInstances{
			client: mgr.GetClient(),
			ctx:    context.Background(),
		}
	*/

	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPAllocation{}).
		//Watches(&source.Kind{Type: &ipamv1alpha1.NetworkInstance{}}, niHandler).
		Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Ipam         ipam.Ipam
	pollInterval time.Duration
	finalizer    *resource.APIFinalizer

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
		// TBD remove finalizer
		if err := r.Ipam.DeAllocateIPPrefix(ctx, ipam.BuildAllocationFromIPAllocation(cr)); err != nil {
			if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
				r.l.Error(err, "cannot delete resource")
				cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
				return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
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
		cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check if the network instance exists in the allocation request
	niName, ok := cr.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey]
	if !ok {
		r.l.Info("cannot allocate prefix, network-intance not found in cr")
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("network-instance not found in cr"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check the network instance existance, to ensure we update the condition in the cr
	// when a network instance get deleted
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      niName,
	}, ni); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		r.l.Info("cannot allocate prefix, network-intance not found")
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("network-instance not found"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check the network instance existance, to ensure we update the condition in the cr
	// when a network instance get deleted
	if meta.WasDeleted(ni) {
		r.l.Info("cannot allocate prefix, network-intance not ready")
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("network-instance not ready"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// for prefixKind network validate if the label exists
	if cr.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindNetwork) {
		_, ok := cr.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkNameKey]
		if !ok {
			r.l.Info("cannot allocate prefix, matchLabels must contain a network key")
			cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("cannot allocate prefix, matchLabels must contain a network key"))
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
	}

	// set origin to label allocation
	if len(cr.Labels) == 0 {
		cr.Labels = map[string]string{}
	}
	cr.Labels[ipamv1alpha1.NephioOriginKey] = string(ipamv1alpha1.OriginIPAllocation)

	allocatedPrefix, err := r.Ipam.AllocateIPPrefix(ctx, ipam.BuildAllocationFromIPAllocation(cr))
	if err != nil {
		r.l.Info("cannot allocate prefix", "err", err)
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed(err.Error()))
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	// if the prefix is allocated in the spec, we need to ensure we get the same allocation
	if cr.Spec.Prefix != "" {
		if allocatedPrefix.AllocatedPrefix != cr.Spec.Prefix {
			// we got a different prefix than requested
			r.l.Error(err, "prefix allocation failed", "requested", cr.Spec.Prefix, "allocated", *allocatedPrefix)
			cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Unknown())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
	}
	cr.Status.Gateway = allocatedPrefix.Gateway
	cr.Status.AllocatedPrefix = allocatedPrefix.AllocatedPrefix
	r.l.Info("Successfully reconciled resource", "allocatedPrefix", *allocatedPrefix)
	cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}
