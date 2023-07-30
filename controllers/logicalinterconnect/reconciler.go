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
	"sort"

	"github.com/go-logr/logr"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	topov1alpha1 "github.com/nokia/k8s-ipam/apis/topo/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlconfig"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/objects"
	"github.com/nokia/k8s-ipam/pkg/resources"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	//reconcileFailed = "reconcile failed"
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
	r.dependencies = []schema.GroupVersionKind{
		invv1alpha1.EndpointGroupVersionKind,
	}

	return nil,
		ctrl.NewControllerManagedBy(mgr).
			Named("LogicalInterconnect").
			For(&topov1alpha1.LogicalInterconnect{}).
			Owns(&invv1alpha1.Link{}).
			Owns(&invv1alpha1.LogicalEndpoint{}).
			Complete(r)
}

type topogvk struct {
	topology string
	schema.GroupVersionKind
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	resource.APIPatchingApplicator
	finalizer *resource.APIFinalizer

	resources resources.Resources

	dependencies []schema.GroupVersionKind
	inventory    map[topogvk]objects.Objects

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
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return reconcile.Result{}, nil
	}

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

	// initializes an inventory for the gvk resources (in this case endpoints per topology)
	if err := r.getDependencyInventory(ctx, cr); err != nil {
		r.l.Error(err, "cannot populate resources")
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

func (r *reconciler) populateResources(ctx context.Context, cr *topov1alpha1.LogicalInterconnect) error {
	var err error
	// initialize the resource list
	r.resources.Init(client.MatchingLabels{})

	// we keep track of all selected enpoints to ensure they get labelled
	sctx, err := r.selectEndpoints(cr)
	if err != nil {
		return err
	}
	// in this case the allocation was successfull
	for _, l := range sctx.getLinkSpecs() {
		linkName := fmt.Sprintf("%s-%s-%s-%s", l.Endpoints[0].NodeName, l.Endpoints[0].InterfaceName, l.Endpoints[1].NodeName, l.Endpoints[1].InterfaceName)
		r.resources.AddNewResource(invv1alpha1.BuildLink(
			metav1.ObjectMeta{
				Name:            linkName,
				Namespace:       cr.Namespace,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
			},
			l,
			invv1alpha1.LinkStatus{},
		))
	}
	// tbd determine multihoming
	for epIdx, lep := range sctx.getLogicalEndpoints() {
		// topology
		r.resources.AddNewResource(invv1alpha1.BuildLogicalEndpoint(
			metav1.ObjectMeta{
				Name:            fmt.Sprintf("%s-logical-ep%d", cr.GetName(), epIdx),
				Namespace:       cr.Namespace,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
			},
			lep,
			invv1alpha1.LogicalEndpointStatus{},
		))
	}

	// TODO label selected endpoints

	return r.resources.APIApply(ctx, cr)
}

func (r *reconciler) selectEndpoints(cr *topov1alpha1.LogicalInterconnect) (*selectionContext, error) {
	sctx := newSelectionContext(cr.Spec.Links)

	// selection is run per logical link endpoint to ensure we take into account topology and node diversity
	for epIdx, ep := range cr.Spec.Endpoints {
		// for node diversity we keep track of the nodes per topology
		// that have been selected - key = topology
		selectedNodes := map[string][]string{}
		// retrieve endpoints per topology - key = topology
		topoEndpoints := map[string][]invv1alpha1.Endpoint{}
		for _, topology := range ep.Topologies {
			var err error
			topoEndpoints[topology], err = r.getTopologyEndpoints(topology, ep.Selector)
			if err != nil {
				return nil, err
			}
			selectedNodes[topology] = []string{"", ""}
		}

		// allocate endpoint per link
		for linkIdx := 0; linkIdx < int(cr.Spec.Links); linkIdx++ {
			// topoIdx is used to find the topology we should use for the
			// endpoint selection
			// Current strategy we walk one by one through each topology
			topoIdx := linkIdx % len(ep.Topologies)
			topology := ep.Topologies[topoIdx]
			// nodeIdx is used for node selection when node diversity is used
			// we assume max 2 nodes for node diversity
			nodeIdx := 0
			// for a multi topology environment node selection is not applicable
			// (we ignore it)
			if len(ep.Topologies) == 1 {
				nodeIdx = linkIdx % 2
			}

			// select an endpoint within a topology
			found := false
			for _, tep := range topoEndpoints[topology] {
				if tep.WasAllocated(topov1alpha1.LogicalInterconnectKindGVKString, cr.Name) {
					found = true
					if err := sctx.addEndpoint(topology, linkIdx, epIdx, tep); err != nil {
						return nil, err
					}
					break
				}

				// if the selectedNode was already selected, node diversity was already
				// checked so we need to allocate an endpoint on the
				selectedNode := selectedNodes[topology][nodeIdx]
				if selectedNode != "" {
					// node selection was already done, we need to select an endpoint
					// on the same node that was not already allocated
					if tep.Spec.NodeName == selectedNode && !tep.IsAllocated(topov1alpha1.LogicalInterconnectKindGVKString, cr.Name) {
						// the nodeName previously selected node matches
						// -> we need to continue searching for an endpoint
						// that matches the previous selected node
						found = true
						if err := sctx.addEndpoint(topology, linkIdx, epIdx, tep); err != nil {
							return nil, err
						}
						break
					}
				} else {
					// node selection is required
					if len(ep.Topologies) == 1 && ep.SelectorPolicy != nil && ep.SelectorPolicy.NodeDiversity != nil && *ep.SelectorPolicy.NodeDiversity > 1 {
						// we need to select a node - ensure the node we select is
						// node diverse from the previous selected nodes and not allocated
						if tep.IsNodeDiverse(nodeIdx, selectedNodes[topology]) && !tep.IsAllocated(topov1alpha1.LogicalInterconnectKindGVKString, cr.Name) {
							// the ep is node diverse and not allocated -> we allocated this endpoint
							found = true
							selectedNodes[topology][nodeIdx] = tep.Spec.NodeName
							if err := sctx.addEndpoint(topology, linkIdx, epIdx, tep); err != nil {
								return nil, err
							}
							break
						}
					} else {
						if !tep.IsAllocated(topov1alpha1.LogicalInterconnectKindGVKString, cr.Name) {
							found = true
							selectedNodes[topology][nodeIdx] = tep.Spec.NodeName
							if err := sctx.addEndpoint(topology, linkIdx, epIdx, tep); err != nil {
								return nil, err
							}
							break
						}
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("no endpoints available")
			}
		}
	}

	return sctx, nil
}

func (r *reconciler) getTopologyEndpoints(topology string, s *metav1.LabelSelector) ([]invv1alpha1.Endpoint, error) {
	tgvk := topogvk{
		GroupVersionKind: invv1alpha1.EndpointGroupVersionKind,
		topology:         topology}
	ul, err := r.inventory[tgvk].GetSelectedObjects(s)
	if err != nil {
		return nil, err
	}
	if len(ul) == 0 {
		return nil, fmt.Errorf("no endpoints found based on selector: %s", s.String())
	}
	eps := []invv1alpha1.Endpoint{}
	for _, u := range ul {
		ep, err := convertUnstructuredToEndpoint(&u)
		if err != nil {
			return nil, err
		}
		eps = append(eps, *ep)
	}

	sort.Slice(eps, func(i, j int) bool {
		return eps[i].Spec.InterfaceName > eps[j].Spec.InterfaceName
	})

	return eps, nil
}

func convertUnstructuredToEndpoint(unstructuredObj *unstructured.Unstructured) (*invv1alpha1.Endpoint, error) {
	// Create a new instance of the typed object (in this case, a Endpoint object)
	typedEndpoint := &invv1alpha1.Endpoint{}

	// Convert the unstructured object to the typed object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, typedEndpoint); err != nil {
		return nil, err
	}

	return typedEndpoint, nil
}

// getDependencyInventory provides an inventory based on a dependency gvk list (endpoints in this case)
// stores this in a map for fast lookup using a gvk + topology
func (r *reconciler) getDependencyInventory(ctx context.Context, cr *topov1alpha1.LogicalInterconnect) error {
	// init nodes
	r.inventory = map[topogvk]objects.Objects{}
	for _, ep := range cr.Spec.Endpoints {
		for _, topology := range ep.Topologies {
			r.l.Info("getDependencyInventory", "topology", topology)
			for _, gvk := range r.dependencies {
				if _, ok := r.inventory[topogvk{GroupVersionKind: gvk, topology: topology}]; !ok {
					o, err := r.listObjectsPerTopology(ctx, gvk, topology)
					if err != nil {
						return err
					}
					for _, obj := range o.GetAllObjects() {
						r.l.Info("getDependencyInventory", "gvk", fmt.Sprintf("%s.%s.%s", obj.GetAPIVersion(), obj.GetKind(), obj.GetName()))
					}
					r.inventory[topogvk{GroupVersionKind: gvk, topology: topology}] = o
				}
			}
		}
	}

	return nil
}

// listObjectsPerTopology list the api server for the gvk resources belonging to a specific topology
func (r *reconciler) listObjectsPerTopology(ctx context.Context, gvk schema.GroupVersionKind, topology string) (objects.Objects, error) {
	opts := []client.ListOption{
		client.MatchingLabels{
			invv1alpha1.NephioTopologyKey: topology,
		},
	}
	objs := meta.GetUnstructuredListFromGVK(&gvk)
	if err := r.List(ctx, objs, opts...); err != nil {
		r.l.Error(err, fmt.Sprintf("cannot list objects: %v", gvk))
		return objects.Objects{}, err
	}
	return objects.Objects{UnstructuredList: *objs}, nil
}
