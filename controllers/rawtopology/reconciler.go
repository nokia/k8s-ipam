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

package rawtopology

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	topov1alpha1 "github.com/nokia/k8s-ipam/apis/topo/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlconfig"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/resources"
	perrors "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	controllers.Register("rawtopologies", &reconciler{})
}

const (
	finalizer = "topo.nephio.org/finalizer"
	// error
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=topo.nephio.org,resources=rawtopologies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=topo.nephio.org,resources=rawtopologies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=topo.nephio.org,resources=rawtopologies/finalizers,verbs=update
//+kubebuilder:rbac:groups=inv.nephio.org,resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=inv.nephio.org,resources=links,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=inv.nephio.org,resources=links/status,verbs=get;update;patch
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
	if err := topov1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	// initialize reconciler
	r.APIPatchingApplicator = resource.NewAPIPatchingApplicator(mgr.GetClient())
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.resources = resources.New(r.APIPatchingApplicator, resources.Config{
		Owns: []schema.GroupVersionKind{
			invv1alpha1.NodeGroupVersionKind,
			invv1alpha1.LinkGroupVersionKind,
		},
	})

	return nil,
		ctrl.NewControllerManagedBy(mgr).
			Named("RawTopologyController").
			For(&topov1alpha1.RawTopology{}).
			Owns(&invv1alpha1.Node{}).
			Owns(&invv1alpha1.Link{}).
			Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	resource.APIPatchingApplicator
	finalizer *resource.APIFinalizer

	resources resources.Resources

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &topov1alpha1.RawTopology{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, errGetCr)
			return ctrl.Result{}, perrors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return reconcile.Result{}, nil
	}

	if meta.WasDeleted(cr) {
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

	if err := r.populateResources(ctx, cr); err != nil {
		// populate resources failed
		if errd := r.resources.APIDelete(ctx, cr); errd != nil {
			err = errors.Join(err, errd)
			r.l.Error(err, "cannot populate and delete existingresources")
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		r.l.Error(err, "cannot populate resources")
		cr.SetConditions(resourcev1alpha1.Failed(err.Error()))
		return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(resourcev1alpha1.Ready())
	return ctrl.Result{}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) populateResources(ctx context.Context, cr *topov1alpha1.RawTopology) error {
	// initialize the resource list + provide the topology key
	r.resources.Init(client.MatchingLabels{
		invv1alpha1.NephioTopologyKey: cr.Name,
	})

	// validate the input
	if err := validate(cr); err != nil {
		return err
	}
	// add the node to the resourceList
	for nodeName, node := range cr.Spec.Nodes {
		r.resources.AddNewResource(buildNode(cr, nodeName, node).DeepCopy())
	}
	for _, l := range cr.Spec.Links {
		r.resources.AddNewResource(buildLink(cr, l, r.getTopologies(l)).DeepCopy())
	}
	return r.resources.APIApply(ctx, cr)
}

func (r *reconciler) getTopologies(l invv1alpha1.LinkSpec) []string {
	topologies := make([]string, 2)
	for ref, o := range r.resources.GetNewResources() {
		n, ok := o.(*invv1alpha1.Node)
		if !ok {
			continue
		}
		if ref.Name == l.Endpoints[0].NodeName {
			topologies[0] = n.Spec.Topology
		}
		if ref.Name == l.Endpoints[1].NodeName {
			topologies[1] = n.Spec.Topology
		}
	}
	return topologies
}

func validate(topo *topov1alpha1.RawTopology) error {
	if err := validateLink2Nodes(topo); err != nil {
		return err
	}

	if err := validateLinks(topo); err != nil {
		return err
	}
	return nil
}

func validateLink2Nodes(topo *topov1alpha1.RawTopology) error {
	invalidNodeRef := []string{}
	for _, l := range topo.Spec.Links {
		for _, e := range l.Endpoints {
			epString := fmt.Sprintf("%s:%s", e.NodeName, e.InterfaceName)
			if _, ok := topo.Spec.Nodes[e.NodeName]; !ok {
				invalidNodeRef = append(invalidNodeRef, epString)
			}
		}
	}
	if len(invalidNodeRef) != 0 {
		return fmt.Errorf("endpoints %q has no node reference", invalidNodeRef)
	}
	return nil
}

func validateLinks(topo *topov1alpha1.RawTopology) error {
	endpoints := map[string]struct{}{}
	// dups accumulates duplicate links
	dups := []string{}
	for _, l := range topo.Spec.Links {
		for _, e := range l.Endpoints {
			epString := fmt.Sprintf("%s:%s", e.NodeName, e.InterfaceName)
			if _, ok := endpoints[epString]; ok {
				dups = append(dups, epString)
			}
			endpoints[epString] = struct{}{}
		}
	}
	if len(dups) != 0 {
		return fmt.Errorf("endpoints %q appeared more than once in the links section of the topology file", dups)
	}
	return nil
}

func buildNode(cr *topov1alpha1.RawTopology, nodeName string, nodeSpec invv1alpha1.NodeSpec) *invv1alpha1.Node {
	labels := map[string]string{}
	for k, v := range nodeSpec.Labels {
		labels[k] = v
	}
	labels[invv1alpha1.NephioTopologyKey] = nodeSpec.GetTopology(cr.Name)
	labels[invv1alpha1.NephioInventoryNodeNameKey] = nodeName
	labels[invv1alpha1.NephioProviderKey] = nodeSpec.Provider

	nodeSpec.Topology = nodeSpec.GetTopology(cr.Name)
	return invv1alpha1.BuildNode(
		metav1.ObjectMeta{
			Name:            nodeName,
			Namespace:       cr.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
		},
		nodeSpec,
		invv1alpha1.NodeStatus{},
	)
}

func buildLink(cr *topov1alpha1.RawTopology, linkSpec invv1alpha1.LinkSpec, topologies []string) *invv1alpha1.Link {
	linkName := fmt.Sprintf("%s-%s-%s-%s",
		linkSpec.Endpoints[0].NodeName,
		linkSpec.Endpoints[0].InterfaceName,
		linkSpec.Endpoints[1].NodeName,
		linkSpec.Endpoints[1].InterfaceName,
	)

	// add the topology to the linkSpec
	linkSpec.Endpoints[0].Topology = topologies[0]
	linkSpec.Endpoints[1].Topology = topologies[1]
	return invv1alpha1.BuildLink(
		metav1.ObjectMeta{
			Name:            linkName,
			Namespace:       cr.Namespace,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
		},
		linkSpec,
		invv1alpha1.LinkStatus{},
	)
}

/*
func getNewResources(cr *topov1alpha1.RawTopology) map[corev1.ObjectReference]client.Object {
	resources := map[corev1.ObjectReference]client.Object{}

	for nodeName, node := range cr.Spec.Nodes {
		n := node
		labels := map[string]string{}
		for k, v := range n.Labels {
			labels[k] = v
		}
		labels[invv1alpha1.NephioTopologyKey] = cr.Name
		labels[invv1alpha1.NephioInventoryNodeNameKey] = nodeName
		labels[invv1alpha1.NephioProviderKey] = n.Provider
		var o client.Object
		o = invv1alpha1.BuildNode(
			metav1.ObjectMeta{
				Name:            nodeName,
				Namespace:       cr.Namespace,
				Labels:          labels,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
			},
			invv1alpha1.NodeSpec{
				UserDefinedLabels: n.UserDefinedLabels,
				Location:          n.Location,
				Provider:          n.Provider,
			},
			invv1alpha1.NodeStatus{},
		)
		resources[corev1.ObjectReference{APIVersion: o.GetResourceVersion(), Kind: o.GetObjectKind().GroupVersionKind().Kind, Name: o.GetName(), Namespace: o.GetNamespace()}] = o

		targetSpec := invv1alpha1.TargetSpec{
			Provider:   n.Provider,
			SecretName: n.Provider,
		}
		if node.Address != nil {
			targetSpec.Address = node.Address
		}
		o = invv1alpha1.BuildTarget(
			metav1.ObjectMeta{
				Name:            nodeName,
				Namespace:       cr.Namespace,
				Labels:          labels,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
			},
			targetSpec,
			invv1alpha1.TargetStatus{},
		)
		resources[corev1.ObjectReference{APIVersion: o.GetResourceVersion(), Kind: o.GetObjectKind().GroupVersionKind().Kind, Name: o.GetName(), Namespace: o.GetNamespace()}] = o

	}
	for _, l := range cr.Spec.Links {
		eps := make([]invv1alpha1.EndpointSpec, 0, 2)

		// define labels - use all the node labels
		labels := map[string]string{}
		labels[invv1alpha1.NephioTopologyKey] = cr.Name
		for k, v := range l.UserDefinedLabels.Labels {
			labels[k] = v
		}
		for _, e := range l.Endpoints {
			for k, v := range cr.Spec.Nodes[e.NodeName].Labels {
				labels[k] = v
			}
			eps = append(eps, invv1alpha1.EndpointSpec{
				NodeName:      e.NodeName,
				InterfaceName: e.InterfaceName,
			})
		}

		linkName := fmt.Sprintf("%s-%s-%s-%s", eps[0].NodeName, eps[0].InterfaceName, eps[1].NodeName, eps[1].InterfaceName)

		for _, e := range l.Endpoints {
			// the endpoint provider is the node provider
			epSpec := invv1alpha1.EndpointSpec{
				InterfaceName: e.InterfaceName,
				NodeName:      e.NodeName,
			}

			epLabels := map[string]string{}
			for k, v := range labels {
				epLabels[k] = v
			}
			epLabels[invv1alpha1.NephioProviderKey] = cr.Spec.Nodes[e.NodeName].Provider
			epLabels[invv1alpha1.NephioInventoryNodeNameKey] = e.NodeName
			epLabels[invv1alpha1.NephioInventoryInterfaceNameKey] = e.InterfaceName
			epLabels[invv1alpha1.NephioInventoryLinkNameKey] = linkName

			o := invv1alpha1.BuildEndpoint(
				metav1.ObjectMeta{
					Name:            fmt.Sprintf("%s-%s", e.NodeName, e.InterfaceName),
					Namespace:       cr.Namespace,
					Labels:          epLabels,
					OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
				},
				epSpec,
				invv1alpha1.EndpointStatus{},
			)
			resources[corev1.ObjectReference{APIVersion: o.APIVersion, Kind: o.Kind, Name: o.Name, Namespace: o.Namespace}] = o
		}

		o := invv1alpha1.BuildLink(
			metav1.ObjectMeta{
				Name:            linkName,
				Namespace:       cr.Namespace,
				Labels:          labels,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: cr.APIVersion, Kind: cr.Kind, Name: cr.Name, UID: cr.UID, Controller: pointer.Bool(true)}},
			},
			invv1alpha1.LinkSpec{
				Endpoints:         eps,
				UserDefinedLabels: l.UserDefinedLabels,
			},
			invv1alpha1.LinkStatus{},
		)
		resources[corev1.ObjectReference{APIVersion: o.APIVersion, Kind: o.Kind, Name: o.Name, Namespace: o.Namespace}] = o
	}
	return resources
}
*/

/*
func (r *reconciler) getExistingResources(ctx context.Context, cr *topov1alpha1.RawTopology) (map[corev1.ObjectReference]client.Object, error) {
	resources := map[corev1.ObjectReference]client.Object{}

	if err := r.listExistingResources(ctx, cr, &invv1alpha1.NodeList{}, resources); err != nil {
		return nil, err
	}
	if err := r.listExistingResources(ctx, cr, &invv1alpha1.LinkList{}, resources); err != nil {
		return nil, err
	}
	if err := r.listExistingResources(ctx, cr, &invv1alpha1.EndpointList{}, resources); err != nil {
		return nil, err
	}
	return resources, nil

}

func (r *reconciler) listExistingResources(ctx context.Context, cr *topov1alpha1.RawTopology, objs ObjectList, resources map[corev1.ObjectReference]client.Object) error {
	opts := []client.ListOption{
		client.MatchingLabels{
			invv1alpha1.NephioTopologyKey: cr.Name,
		},
	}
	if err := r.List(ctx, objs, opts...); err != nil {
		return err
	}
	for _, o := range objs.GetItems() {
		for _, ref := range o.GetOwnerReferences() {
			if ref.UID == cr.UID {
				resources[corev1.ObjectReference{APIVersion: o.GetResourceVersion(), Kind: o.GetObjectKind().GroupVersionKind().Kind, Name: o.GetName(), Namespace: o.GetNamespace()}] = o
			}
		}
	}
	return nil
}

type ObjectList interface {
	client.ObjectList

	GetItems() []client.Object
}
*/
