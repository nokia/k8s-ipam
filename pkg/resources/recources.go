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

package resources

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	"github.com/nokia/k8s-ipam/pkg/meta"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Resources interface {
	Init(ml client.MatchingLabels)
	AddNewResource(o client.Object)
	APIApply(ctx context.Context, cr client.Object) error
	APIDelete(ctx context.Context, cr client.Object) error
	GetNewResources() map[corev1.ObjectReference]client.Object
}

type Config struct {
	Owns []schema.GroupVersionKind
}

func New(c resource.APIPatchingApplicator, cfg Config) Resources {
	return &resources{
		APIPatchingApplicator: c,
		cfg:                   cfg,
		newResources:          map[corev1.ObjectReference]client.Object{},
		existingResources:     map[corev1.ObjectReference]client.Object{},
	}
}

type resources struct {
	resource.APIPatchingApplicator
	cfg               Config
	m                 sync.RWMutex
	newResources      map[corev1.ObjectReference]client.Object
	existingResources map[corev1.ObjectReference]client.Object
	matchLabels       client.MatchingLabels

	l logr.Logger
}

// Init initializes the new and exisiting resource inventory list
func (r *resources) Init(ml client.MatchingLabels) {
	r.matchLabels = ml
	r.newResources = map[corev1.ObjectReference]client.Object{}
	r.existingResources = map[corev1.ObjectReference]client.Object{}
}

// AddNewResource adds a new resource to the inventoru
func (r *resources) AddNewResource(o client.Object) {
	r.m.Lock()
	defer r.m.Unlock()
	r.newResources[corev1.ObjectReference{
		APIVersion: o.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       o.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  o.GetNamespace(),
		Name:       o.GetName(),
	}] = o
}

// GetExistingResources retrieves the exisiting resource that match the label selector and the owner reference
// and puts the results in the resource inventory
func (r *resources) getExistingResources(ctx context.Context, cr client.Object) error {
	//r.m.Lock()
	//defer r.m.Unlock()

	for _, gvk := range r.cfg.Owns {
		objs := meta.GetUnstructuredListFromGVK(&gvk)

		opts := []client.ListOption{}
		if len(r.matchLabels) > 0 {
			opts = append(opts, r.matchLabels)
		}

		if err := r.List(ctx, objs, opts...); err != nil {
			return err
		}
		for _, o := range objs.Items {
			for _, ref := range o.GetOwnerReferences() {
				if ref.UID == cr.GetUID() {
					r.existingResources[corev1.ObjectReference{APIVersion: o.GetAPIVersion(), Kind: o.GetKind(), Name: o.GetName(), Namespace: o.GetNamespace()}] = &o
				}
			}
		}
	}
	return nil
}

// APIDelete is used to delete the existing resources that are owned by this cr
// the implementation retrieves the existing resources and deletes them
func (r *resources) APIDelete(ctx context.Context, cr client.Object) error {
	r.m.Lock()
	defer r.m.Unlock()

	r.l = log.FromContext(ctx)
	r.l.Info("api delete")

	// step 0: get existing resources
	if err := r.getExistingResources(ctx, cr); err != nil {
		return err
	}
	for _, o := range r.existingResources {
		if err := r.Delete(ctx, o); err != nil {
			r.l.Info("api delete", "error", err, "object", o)
			return err
		}
	}
	return nil
}

// APIApply
// step 0: get existing resources
// step 1: remove the exisiting resources from the internal resource list that overlap with the new resources
// step 2: delete the exisiting resources that are no longer needed
// step 3: apply the new resources to the api server
func (r *resources) APIApply(ctx context.Context, cr client.Object) error {
	r.m.Lock()
	defer r.m.Unlock()

	r.l = log.FromContext(ctx)
	r.l.Info("api apply")
	// step 0: get existing resources
	if err := r.getExistingResources(ctx, cr); err != nil {
		return err
	}

	// step 1: remove the exisiting resources that overlap with the new resources
	// since apply will change the content.
	for ref := range r.newResources {
		delete(r.existingResources, ref)
	}

	r.l.Info("api apply existing resources to be deleted", "existing resources", r.getExistingRefs())

	// step2b delete the exisiting resource that are no longer needed
	for _, o := range r.existingResources {
		if err := r.Delete(ctx, o); err != nil {
			r.l.Info("api apply", "error", err, "object", o)
			return err
		}
	}

	// step3b apply the new resources to the api server
	for _, o := range r.newResources {
		if err := r.Apply(ctx, o); err != nil {
			return err
		}
	}
	return nil
}

func (r *resources) GetNewResources() map[corev1.ObjectReference]client.Object {
	r.m.RLock()
	defer r.m.RUnlock()

	return r.newResources
}

func (r *resources) getNewRefs() []corev1.ObjectReference {
	l := []corev1.ObjectReference{}
	for ref := range r.newResources {
		l = append(l, ref)
	}
	return l
}

func (r *resources) getExistingRefs() []corev1.ObjectReference {
	l := []corev1.ObjectReference{}
	for ref := range r.existingResources {
		l = append(l, ref)
	}
	return l
}
