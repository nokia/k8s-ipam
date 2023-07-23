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

package interconnect

import (
	"context"
	"fmt"

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
	controllers.Register("interconnects", &reconciler{})
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
			Named("Interconnect").
			For(&topov1alpha1.Interconnect{}).
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

	cr := &topov1alpha1.Interconnect{}
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

func (r *reconciler) populateResources(ctx context.Context, cr *topov1alpha1.Interconnect) error {
	// initialize the resource list
	r.resources.Init(client.MatchingLabels{})

	// check ready state
	selectedEndpoints := []invv1alpha1.Endpoint{}
	logicalEndpoints := map[int][]invv1alpha1.LogicalEndpointSpec{}
	for linkIdx, link := range cr.Spec.Links {
		// getLinkSpecs initializes the links based on links
		// in an abstracted link we can have multiple links within an
		// abstracted link
		linkSpecs := getLinkSpecs(link.Links)
		isLogicalLink := false
		llIndex := cr.GetLogicalLinkIndex(linkIdx)
		if llIndex != -1 {
			isLogicalLink = true
			if _, ok := logicalEndpoints[llIndex]; !ok {
				logicalEndpoints[llIndex] = make([]invv1alpha1.LogicalEndpointSpec, 2)
			}
		}

		for epIdx, ep := range link.Endpoints {
			topology, err := cr.GetTopology(linkIdx, epIdx)
			if err != nil {
				return err
			}
			s, err := cr.GetEndpointSelector(linkIdx, epIdx)
			if err != nil {
				return err
			}
			// we will always get 1 endpoint back,
			// otherwise we get an error
			seps, err := r.getEndpoints(topology, s)
			if err != nil {
				return err
			}
			if link.Links != nil {
				// logical link
				if *link.Links > uint32(len(seps)) {
					return fmt.Errorf("cannot allocate logical interconect links, no available eps for linkidx: %d, epidx: %d", linkIdx, epIdx)
				}
				var found bool
				for _, sep := range seps {
					if cr.IsAllocated(sep.Labels) {
						continue
					}
					// sort the endpoints
					// if multi-node provide the list per node and sort per interface-index
					// if not multi-node sort the list per
				}
				if !found {
					return fmt.Errorf("cannot allocate physical interconect links, no available eps for linkidx: %d, epidx: %d", linkIdx, epIdx)
				}

			} else {
				// physical link, we pick the first available ep that is not allocated
				// by another resource
				var found bool
				for _, sep := range seps {
					// validate if this link was not already allocated/ used by something
					// else
					if cr.IsAllocated(sep.Labels) {
						continue
					}
					found = true
					linkSpecs[0].Endpoints[epIdx] = invv1alpha1.EndpointSpec{
						InterfaceName: sep.Spec.InterfaceName,
						NodeName:      sep.Spec.NodeName,
					}
					selectedEndpoints = append(selectedEndpoints, sep)

					if isLogicalLink {
						if len(logicalEndpoints[llIndex][epIdx].Endpoints) == 0 {
							logicalEndpoints[llIndex][epIdx].Endpoints = make([]invv1alpha1.EndpointSpec, 0)
						}
						logicalEndpoints[llIndex][epIdx].Endpoints = append(logicalEndpoints[llIndex][epIdx].Endpoints, sep.Spec)
						if ep.LogicalEndpointName != nil {
							logicalName := *ep.LogicalEndpointName
							logicalEndpoints[llIndex][epIdx].LagName = &logicalName
							if link.Lacp != nil {
								lacp := *link.Lacp
								logicalEndpoints[llIndex][epIdx].Lacp = &lacp
							}
						}
					}
					break

				}
				if !found {
					return fmt.Errorf("cannot allocate physical interconect links, no available eps for linkidx: %d, epidx: %d", linkIdx, epIdx)
				}
			}
		}
		for _, l := range linkSpecs {
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
		// determine multihoming
		for _, leps := range logicalEndpoints {
			for _, lep := range leps {
				// topology
				r.resources.AddNewResource(invv1alpha1.BuildLogicalEndpoint(
					metav1.ObjectMeta{
						Name:            "tbd",
						Namespace:       cr.Namespace,
						OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
					},
					lep,
					invv1alpha1.LogicalEndpointStatus{},
				))
			}
		}

	}
	// TODO label selected endpoints

	return r.resources.APIApply(ctx, cr)
}

func getLinkSpecs(links *uint32) []invv1alpha1.LinkSpec {
	linkSpecs := []invv1alpha1.LinkSpec{}
	if links == nil {
		linkSpecs = append(linkSpecs, invv1alpha1.LinkSpec{
			Endpoints: make([]invv1alpha1.EndpointSpec, 0, 2),
		})
	} else {
		for i := 0; i <= int(*links); i++ {
			linkSpecs = append(linkSpecs, invv1alpha1.LinkSpec{
				Endpoints: make([]invv1alpha1.EndpointSpec, 0, 2),
			})
		}
	}
	return linkSpecs
}

func (r *reconciler) getEndpoints(topology string, s *metav1.LabelSelector) ([]invv1alpha1.Endpoint, error) {
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

// getDependencyInventory provides an inventory based on a dependency gvk list (nodes, endpoints in this case)
// stores this in a map for fast lookup using a gvk + topology
func (r *reconciler) getDependencyInventory(ctx context.Context, cr *topov1alpha1.Interconnect) error {
	// init nodes
	r.inventory = map[topogvk]objects.Objects{}
	for linkIdx, link := range cr.Spec.Links {
		for epIdx := range link.Endpoints {
			for _, gvk := range r.dependencies {
				topology, err := cr.GetTopology(linkIdx, epIdx)
				if err != nil {
					return err
				}
				// only get the objects if the gvk topo has not yet been requested
				if _, ok := r.inventory[topogvk{GroupVersionKind: gvk, topology: topology}]; !ok {
					o, err := r.listObjectsPerTopology(ctx, gvk, topology)
					if err != nil {
						return err
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
