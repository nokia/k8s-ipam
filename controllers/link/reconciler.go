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

package link

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	topov1alpha1 "github.com/nokia/k8s-ipam/apis/topo/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlconfig"
	"github.com/nokia/k8s-ipam/pkg/lease"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/objects/endpoint"
	perrors "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	controllers.Register("links", &reconciler{})
}

const (
	finalizer = "topo.nephio.org/finalizer"
	// error
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"
)

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) Setup(ctx context.Context, mgr ctrl.Manager, cfg *ctrlconfig.ControllerConfig) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {
	// register scheme
	if err := invv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	// initialize reconciler
	r.APIPatchingApplicator = resource.NewAPIPatchingApplicator(mgr.GetClient())
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)

	r.endpoint = endpoint.New(mgr.GetClient())
	r.epLease = lease.New(mgr.GetClient(), types.NamespacedName{Namespace: os.Getenv("POD_NAMESPACE"), Name: "endpoint"})

	return nil,
		ctrl.NewControllerManagedBy(mgr).
			Named("LinkController").
			For(&invv1alpha1.Link{}).
			Watches(&invv1alpha1.Endpoint{}, &endpointEventHandler{client: mgr.GetClient()}).
			Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	resource.APIPatchingApplicator
	finalizer *resource.APIFinalizer

	endpoint endpoint.Endpoint
	epLease  lease.Lease

	claimedEndpoints []invv1alpha1.Endpoint

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &invv1alpha1.Link{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, errGetCr)
			return ctrl.Result{}, perrors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return reconcile.Result{}, nil
	}

	// for links owned by the logical interconnect link we dont do anything
	for _, ownRef := range cr.OwnerReferences {
		if ownRef.APIVersion == topov1alpha1.GroupVersion.String() &&
			ownRef.Kind == topov1alpha1.LogicalInterconnectKind {
			return reconcile.Result{}, nil
		}
	}

	// initialize claimed endpoints
	var err error
	r.claimedEndpoints, err = r.endpoint.GetClaimedEndpoints(ctx, cr)
	if err != nil {
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if meta.WasDeleted(cr) {
		// delete usedRef from endpoint status
		if err := r.endpoint.DeleteClaim(ctx, r.claimedEndpoints); err != nil {
			r.l.Error(err, "cannot delete endpoint claim")
			cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		r.l.Info("Successfully deleted resource")
		return reconcile.Result{Requeue: false}, nil
	}
	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		r.l.Error(err, "cannot add finalizer")
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr = cr.DeepCopy()
	// acquire endpoint lease to update the resource
	if err := r.epLease.AcquireLease(ctx, cr); err != nil {
		r.l.Error(err, "cannot acquire endpoint lease")
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true, RequeueAfter: lease.RequeueInterval}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.claimResources(ctx, cr); err != nil {
		// claim resources failed
		if errd := r.endpoint.DeleteClaim(ctx, r.claimedEndpoints); errd != nil {
			err = errors.Join(err, errd)
			r.l.Error(err, "cannot claim resource and delete endpoint claims")
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		r.l.Error(err, "cannot claim resources")
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(resourcev1alpha1.Ready())
	return ctrl.Result{}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) claimResources(ctx context.Context, cr *invv1alpha1.Link) error {
	r.l.Info("claim resources")

	// LogicalInterconnectKind provides its own claim logic
	// so we ignore links which such owner reference
	for _, ownRef := range cr.OwnerReferences {
		if ownRef.APIVersion == topov1alpha1.GroupVersion.String() &&
			ownRef.Kind == topov1alpha1.LogicalInterconnectKind {
			return nil
		}
	}

	// initialize the endpoint topologies
	if err := r.endpoint.Init(ctx, cr.GetTopologies()); err != nil {
		return err
	}

	// claim the endpoints
	endpoints := []invv1alpha1.Endpoint{}
	for _, ep := range cr.Spec.Endpoints {
		eps, err := r.endpoint.GetTopologyEndpointsWithSelector(ep.Topology, &metav1.LabelSelector{
			MatchLabels: map[string]string{
				invv1alpha1.NephioInventoryInterfaceNameKey: ep.InterfaceName,
				invv1alpha1.NephioInventoryNodeNameKey:      ep.NodeName},
		})
		if err != nil {
			return err
		}
		r.l.Info("link claimResources", "nbr endpoints", len(eps), "endpoint", eps[0].Name)
		if len(eps) != 1 {
			return fmt.Errorf("expecting 1 endpoint, got: %d, data: %v", len(eps), eps)
		}
		if eps[0].IsAllocated(cr) {
			return fmt.Errorf("endpoint allocated by another resource: %v", eps[0].Status.ClaimRef)
		}
		endpoints = append(endpoints, eps[0])
	}

	// identify claims to be deleted
	tobeDeletedClaims := []invv1alpha1.Endpoint{}
	for _, cep := range r.claimedEndpoints {
		found := false
		for _, ep := range endpoints {
			if cep.Name == ep.Name && cep.Namespace == ep.Namespace {
				found = true
				break
			}
		}
		if !found {
			tobeDeletedClaims = append(tobeDeletedClaims, cep)
		}
	}
	// updated the claim ref in the selected endpoints
	if err := r.endpoint.DeleteClaim(ctx, tobeDeletedClaims); err != nil {
		return err
	}

	// updated the claim ref in the selected endpoints
	if err := r.endpoint.Claim(ctx, cr, endpoints); err != nil {
		return err
	}

	return nil
}
