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
	"github.com/nokia/k8s-ipam/pkg/resources"
	perrors "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	controllers.Register("logicalinterconnects", &reconciler{})
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
	if err := topov1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	// initialize reconciler
	r.APIPatchingApplicator = resource.NewAPIPatchingApplicator(mgr.GetClient())
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.resources = resources.New(r.APIPatchingApplicator, resources.Config{
		Owns: []schema.GroupVersionKind{
			invv1alpha1.LinkGroupVersionKind,
			invv1alpha1.LogicalEndpointGroupVersionKind,
		},
	})
	r.endpoint = endpoint.New(mgr.GetClient())
	r.epLease = lease.New(mgr.GetClient(), types.NamespacedName{Namespace: os.Getenv("POD_NAMESPACE"), Name: "endpoint"})

	return nil,
		ctrl.NewControllerManagedBy(mgr).
			Named("LogicalInterconnectController").
			For(&topov1alpha1.LogicalInterconnect{}).
			Owns(&invv1alpha1.Link{}).
			Owns(&invv1alpha1.LogicalEndpoint{}).
			Watches(&invv1alpha1.Endpoint{}, &endpointEventHandler{client: mgr.GetClient()}).
			Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	resource.APIPatchingApplicator
	finalizer *resource.APIFinalizer

	resources resources.Resources
	endpoint  endpoint.Endpoint
	epLease     lease.Lease

	claimedEndpoints []invv1alpha1.Endpoint

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &topov1alpha1.LogicalInterconnect{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, errGetCr)
			return ctrl.Result{}, perrors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return reconcile.Result{}, nil
	}
	var err error
	r.claimedEndpoints, err = r.endpoint.GetClaimedEndpoints(ctx, cr)
	if err != nil {
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true, RequeueAfter: lease.RequeueInterval}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr = cr.DeepCopy()

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

	// acquire lease to update the resource
	if err := r.epLease.AcquireLease(ctx, cr); err != nil {
		r.l.Error(err, "cannot acquire lease")
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.populateResources(ctx, cr); err != nil {
		// populate resources failed
		if errd := r.resources.APIDelete(ctx, cr); errd != nil {
			err = errors.Join(err, errd)
			r.l.Error(err, "cannot populate and delete existingresources")
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		if errd := r.endpoint.DeleteClaim(ctx, r.claimedEndpoints); errd != nil {
			err = errors.Join(err, errd)
			r.l.Error(err, "cannot populate and delete endpoint claims")
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		r.l.Error(err, "cannot populate resources")
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(resourcev1alpha1.Ready())
	return ctrl.Result{}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) populateResources(ctx context.Context, cr *topov1alpha1.LogicalInterconnect) error {
	r.l.Info("populate resources")
	var err error
	// initialize the resource list
	r.resources.Init(client.MatchingLabels{})

	// we keep track of all selected enpoints to ensure they get labelled
	s := newSelector(cr.Spec.Links, r.endpoint)
	selectionResult, err := s.selectEndpoints(ctx, cr)
	if err != nil {
		return err
	}
	// in this case the allocation was successfull
	for _, l := range selectionResult.getLinkSpecs() {
		linkName := fmt.Sprintf("%s-%s-%s-%s", l.Endpoints[0].NodeName, l.Endpoints[0].InterfaceName, l.Endpoints[1].NodeName, l.Endpoints[1].InterfaceName)
		r.resources.AddNewResource(invv1alpha1.BuildLink(
			metav1.ObjectMeta{
				Name:            linkName,
				Namespace:       cr.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
			},
			l,
			invv1alpha1.LinkStatus{},
		).DeepCopy())
	}
	// tbd determine multihoming
	for epIdx, lep := range selectionResult.getLogicalEndpoints() {
		// topology
		r.resources.AddNewResource(invv1alpha1.BuildLogicalEndpoint(
			metav1.ObjectMeta{
				Name:            fmt.Sprintf("%s-logical-ep%d", cr.GetName(), epIdx),
				Namespace:       cr.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
			},
			lep,
			invv1alpha1.LogicalEndpointStatus{},
		).DeepCopy())
	}

	// identify claims to be deleted
	tobeDeletedClaims := []invv1alpha1.Endpoint{}
	for _, cep := range r.claimedEndpoints {
		found := false
		for _, ep := range selectionResult.getSelectedEndpoints() {
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
	if err := r.endpoint.Claim(ctx, cr, selectionResult.getSelectedEndpoints()); err != nil {
		return err
	}

	return r.resources.APIApply(ctx, cr)
}
