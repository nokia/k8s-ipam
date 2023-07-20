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
	"fmt"

	"github.com/go-logr/logr"
	"github.com/henderiw-nephio/network-node-operator/pkg/node"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlconfig"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/resources"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	controllers.Register("nodes", &reconciler{})
}

const (
	finalizer = "inv.nephio.org/finalizer"
	// error
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=inv.nephio.org,resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=inv.nephio.org,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=endpoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=inv.nephio.org,resources=targets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=targets/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) Setup(ctx context.Context, mgr ctrl.Manager, cfg *ctrlconfig.ControllerConfig) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {
	// register scheme
	if err := invv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	// initialize reconciler
	r.APIPatchingApplicator = resource.NewAPIPatchingApplicator(mgr.GetClient())
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.resources = resources.New(r.APIPatchingApplicator, resources.Config{
		Owns: []schema.GroupVersionKind{
			invv1alpha1.EndpointGroupVersionKind,
			invv1alpha1.TargetGroupVersionKind,
		},
	})
	r.nodeRegistry = cfg.Noderegistry
	r.scheme = mgr.GetScheme()

	return nil,
		ctrl.NewControllerManagedBy(mgr).
			Named("Node").
			For(&invv1alpha1.Node{}).
			Owns(&invv1alpha1.Endpoint{}).
			Owns(&invv1alpha1.Target{}).
			Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	resource.APIPatchingApplicator
	finalizer *resource.APIFinalizer

	resources    resources.Resources
	scheme       *runtime.Scheme
	nodeRegistry node.NodeRegistry

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &invv1alpha1.Node{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, errGetCr)
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return reconcile.Result{}, nil
	}

	cr = cr.DeepCopy()
	if meta.WasDeleted(cr) {
		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
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
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.populateResources(ctx, cr); err != nil {
		r.l.Error(err, "cannot populate resources")
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	cr.SetConditions(resourcev1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) populateResources(ctx context.Context, cr *invv1alpha1.Node) error {
	// initialize the resource list
	r.resources.Init()
	// build target CR and add it to the resource inventory
	r.resources.AddNewResource(buildTarget(cr))

	// get the specific provider implementation of the network device
	node, err := r.nodeRegistry.NewNodeOfProvider(cr.Spec.Provider, r.Client, r.scheme)
	if err != nil {
		return err
	}
	// get the node config associated to the node
	nc, err := node.GetNodeConfig(ctx, cr)
	if err != nil {
		return err
	}
	// get interfaces
	itfces, err := node.GetInterfaces(nc)
	if err != nil {
		return err
	}
	// build endpoints based on the node model
	for _, itfce := range itfces {
		r.resources.AddNewResource(buildEndpoint(cr, itfce))
	}

	return r.resources.APIApply(ctx)
}

func buildTarget(cr *invv1alpha1.Node) *invv1alpha1.Target {
	labels := cr.GetLabels()
	for k, v := range cr.Spec.GetUserDefinedLabels() {
		labels[k] = v
	}
	targetSpec := invv1alpha1.TargetSpec{
		ParametersRef: cr.Spec.ParametersRef,
		Provider:      cr.Spec.Provider,
		SecretName:    cr.Spec.Provider,
	}
	if cr.Spec.Address != nil {
		targetSpec.Address = cr.Spec.Address
	}
	return invv1alpha1.BuildTarget(
		metav1.ObjectMeta{
			Name:            cr.GetName(),
			Namespace:       cr.GetNamespace(),
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
		},
		targetSpec,
		invv1alpha1.TargetStatus{},
	)
}

func buildEndpoint(cr *invv1alpha1.Node, itfce node.Interface) *invv1alpha1.Endpoint {
	labels := map[string]string{}
	labels[invv1alpha1.NephioTopologyKey] = cr.GetLabels()[invv1alpha1.NephioTopologyKey]
	labels[invv1alpha1.NephioProviderKey] = cr.GetLabels()[invv1alpha1.NephioProviderKey]
	labels[invv1alpha1.NephioNodeNameKey] = cr.GetLabels()[invv1alpha1.NephioNodeNameKey]
	labels[invv1alpha1.NephioInterfaceNameKey] = itfce.Name
	for k, v := range cr.Spec.GetUserDefinedLabels() {
		labels[k] = v
	}
	epSpec := invv1alpha1.EndpointSpec{
		EndpointProperties: invv1alpha1.EndpointProperties{
			NodeName:      cr.GetName(),
			InterfaceName: itfce.Name,
		},
		Provider: invv1alpha1.Provider{
			Provider: cr.Spec.Provider,
		},
	}

	return invv1alpha1.BuildEndpoint(
		metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s", cr.GetName(), itfce.Name),
			Namespace:       cr.GetNamespace(),
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
		},
		epSpec,
		invv1alpha1.EndpointStatus{},
	)
}
