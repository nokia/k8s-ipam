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

package endpoint

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/go-logr/logr"
	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/objects"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Endpoint interface {
	Init(ctx context.Context, topologies []string) error
	GetClaimedEndpoints(ctx context.Context, o client.Object) ([]invv1alpha1.Endpoint, error)
	GetTopologyEndpointsWithSelector(topology string, s *metav1.LabelSelector) ([]invv1alpha1.Endpoint, error)
	Claim(ctx context.Context, o client.Object, eps []invv1alpha1.Endpoint) error
	DeleteClaim(ctx context.Context, eps []invv1alpha1.Endpoint) error
}

func New(c client.Client) Endpoint {
	return &endpoint{
		Client: c,
	}
}

type endpoint struct {
	client.Client
	l logr.Logger

	inventory map[string]objects.Objects
}

func getCoreRef(o client.Object) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: o.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       o.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  o.GetNamespace(),
		Name:       o.GetName(),
	}
}

// Init initializes the endpoint inventory based on topology data
func (r *endpoint) Init(ctx context.Context, topologies []string) error {
	r.l = log.FromContext(ctx)
	// reset the topology information
	r.inventory = map[string]objects.Objects{}
	// initialize the inventory with the topology data
	for _, topology := range topologies {
		// if the topology is already listed we can avoid retreiving the data again
		if _, ok := r.inventory[topology]; !ok {
			eps, err := r.listEndpointsPerTopology(ctx, topology)
			if err != nil {
				return err
			}
			r.inventory[topology] = eps
		}
	}

	return nil
}

func (r *endpoint) listEndpointsPerTopology(ctx context.Context, topology string) (objects.Objects, error) {
	r.l = log.FromContext(ctx)
	opts := []client.ListOption{
		client.MatchingLabels{
			invv1alpha1.NephioTopologyKey: topology,
		},
	}
	objs := &invv1alpha1.EndpointList{}
	if err := r.List(ctx, objs, opts...); err != nil {
		return objects.Objects{}, err
	}

	return objects.Objects{ExtObjects: objs}, nil
}

func (r *endpoint) GetClaimedEndpoints(ctx context.Context, o client.Object) ([]invv1alpha1.Endpoint, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("getClaimedEndpoints", "req", getCoreRef(o).String())

	eps := []invv1alpha1.Endpoint{}

	epl := &invv1alpha1.EndpointList{}
	if err := r.List(ctx, epl); err != nil {
		return nil, err
	}
	for _, ep := range epl.Items {
		if ep.Status.ClaimRef != nil &&
			ep.Status.ClaimRef.APIVersion == o.GetObjectKind().GroupVersionKind().GroupVersion().String() &&
			ep.Status.ClaimRef.Kind == o.GetObjectKind().GroupVersionKind().Kind &&
			ep.Status.ClaimRef.Name == o.GetName() &&
			ep.Status.ClaimRef.Namespace == o.GetNamespace() {
			eps = append(eps, ep)
		}
	}
	return eps, nil
}

func (r *endpoint) Claim(ctx context.Context, o client.Object, eps []invv1alpha1.Endpoint) error {
	r.l = log.FromContext(ctx)
	for _, ep := range eps {
		ep.Status.ClaimRef = getCoreRef(o)
		if err := r.Status().Update(ctx, &ep); err != nil {
			return err
		}
	}
	return nil
}

func (r *endpoint) DeleteClaim(ctx context.Context, eps []invv1alpha1.Endpoint) error {
	r.l = log.FromContext(ctx)
	for _, ep := range eps {
		ep.Status.ClaimRef = nil
		if err := r.Status().Update(ctx, &ep); err != nil {
			return err
		}
	}
	return nil
}

func (r *endpoint) GetTopologyEndpointsWithSelector(topology string, s *metav1.LabelSelector) ([]invv1alpha1.Endpoint, error) {
	if _, ok := r.inventory[topology]; !ok {
		return nil, fmt.Errorf("topology %s not found in inventory", topology)
	}

	seps, err := r.inventory[topology].GetSelectedObjects(s)
	if err != nil {
		return nil, err
	}
	if len(seps) == 0 {
		return nil, fmt.Errorf("no endpoints found based on selector: %s", s.String())
	}
	eps := []invv1alpha1.Endpoint{}
	for _, sep := range seps {
		ep, ok := sep.(*invv1alpha1.Endpoint)
		if !ok {
			return nil, fmt.Errorf("GetTopologyEndpointsWithSelector wrong type: %v", reflect.TypeOf(sep).Name())
		}
		eps = append(eps, *ep)
	}

	sort.Slice(eps, func(i, j int) bool {
		return eps[i].Spec.InterfaceName < eps[j].Spec.InterfaceName
	})

	return eps, nil
}
