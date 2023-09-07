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

	//kresource "github.com/nephio-project/nephio/controllers/pkg/resource"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Resources interface {
	Init(ml client.MatchingLabels)
	AddNewResource(cr client.Object, o client.Object) error
	APIApply(ctx context.Context, cr client.Object) error
	APIDelete(ctx context.Context, cr client.Object) error
	GetNewResources() map[corev1.ObjectReference]client.Object
}

type Config struct {
	OwnerRef bool
	Owns     []schema.GroupVersionKind
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
func (r *resources) AddNewResource(cr, o client.Object) error {
	r.m.Lock()
	defer r.m.Unlock()

	if r.cfg.OwnerRef {
		o.SetOwnerReferences([]v1.OwnerReference{
			{
				APIVersion: cr.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       cr.GetObjectKind().GroupVersionKind().Kind,
				Name:       cr.GetName(),
				UID:        cr.GetUID(),
				Controller: ptr.To(true),
			},
		})
	} else {
		labels := o.GetLabels()
		if len(labels) == 0 {
			labels = map[string]string{}
		}
		ownref := &meta.OwnerRef{
			APIVersion: cr.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       cr.GetObjectKind().GroupVersionKind().Kind,
			Name:       cr.GetName(),
			Namespace:  cr.GetNamespace(),
			UID:        cr.GetUID(),
		}

		labels[resourcev1alpha1.NephioOwnerRefKey] = ownref.String()
		o.SetLabels(labels)
	}

	r.newResources[corev1.ObjectReference{
		APIVersion: o.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       o.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  o.GetNamespace(),
		Name:       o.GetName(),
	}] = o
	return nil
}

// GetExistingResources retrieves the exisiting resource that match the label selector and the owner reference
// and puts the results in the resource inventory
func (r *resources) getExistingResources(ctx context.Context, cr client.Object) error {
	//r.m.Lock()
	//defer r.m.Unlock()

	for _, gvk := range r.cfg.Owns {
		gvk := gvk
		objs := meta.GetUnstructuredListFromGVK(&gvk)

		opts := []client.ListOption{}
		if len(r.matchLabels) > 0 {
			opts = append(opts, r.matchLabels)
		}

		if err := r.List(ctx, objs, opts...); err != nil {
			return err
		}
		for _, o := range objs.Items {
			o := o
			if r.cfg.OwnerRef {
				for _, ref := range o.GetOwnerReferences() {
					if ref.UID == cr.GetUID() {
						r.existingResources[corev1.ObjectReference{APIVersion: o.GetAPIVersion(), Kind: o.GetKind(), Name: o.GetName(), Namespace: o.GetNamespace()}] = &o
					}
				}
			} else {
				for k, v := range o.GetLabels() {
					if k == resourcev1alpha1.NephioOwnerRefKey {
						ownref := &meta.OwnerRef{
							APIVersion: cr.GetObjectKind().GroupVersionKind().GroupVersion().String(),
							Kind:       cr.GetObjectKind().GroupVersionKind().Kind,
							Name:       cr.GetName(),
							Namespace:  cr.GetNamespace(),
							UID:        cr.GetUID(),
						}
						if v == ownref.String() {
							r.existingResources[corev1.ObjectReference{APIVersion: o.GetAPIVersion(), Kind: o.GetKind(), Name: o.GetName(), Namespace: o.GetNamespace()}] = &o
						}
					}
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
	return r.apiDelete(ctx, cr)
}

func (r *resources) apiDelete(ctx context.Context, cr client.Object) error {
	// delete in priority
	for ref, o := range r.existingResources {
		ref := ref
		o := o
		r.l.Info("api delete existing resource", "referernce", ref.String())
		if ref.Kind == "Namespace" {
			continue
		}
		if err := r.Delete(ctx, o); err != nil {
			r.l.Info("api delete delete", "referernce", ref.String(), "nsn", types.NamespacedName{Name: o.GetName(), Namespace: o.GetNamespace()}.String())
			if resource.IgnoreNotFound(err) != nil {
				r.l.Info("api delete", "error", err, "object", o)
				return err
			}
			delete(r.existingResources, ref)
		}
	}
	for ref, o := range r.existingResources {
		ref := ref
		o := o
		if err := r.Delete(ctx, o); err != nil {
			if resource.IgnoreNotFound(err) != nil {
				r.l.Info("api delete", "error", err, "object", o)
				return err
			}
			delete(r.existingResources, ref)
		}
	}
	return nil
}

func (r *resources) apiCreate(ctx context.Context, cr client.Object) error {
	// apply in priority
	for ref, o := range r.newResources {
		ref := ref
		o := o
		if ref.Kind == "Namespace" {
			r.l.Info("api apply", "object", o)
			if err := r.Apply(ctx, o); err != nil {
				return err
			}
			delete(r.newResources, ref)
		} else {
			continue
		}
	}
	for _, o := range r.newResources {
		o := o
		r.l.Info("api apply", "object", o)
		if err := r.Apply(ctx, o); err != nil {
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
	if err := r.apiDelete(ctx, cr); err != nil {
		return err
	}

	// step3b apply the new resources to the api server
	return r.apiCreate(ctx, cr)
}

func (r *resources) GetNewResources() map[corev1.ObjectReference]client.Object {
	r.m.RLock()
	defer r.m.RUnlock()

	res := make(map[corev1.ObjectReference]client.Object, len(r.newResources))
	for ref, o := range r.newResources {
		ref:= ref
		o := o
		res[ref] = o
	}
	return res
}

/*
func (r *resources) getNewRefs() []corev1.ObjectReference {
	l := []corev1.ObjectReference{}
	for ref := range r.newResources {
		l = append(l, ref)
	}
	return l
}
*/

func (r *resources) getExistingRefs() []corev1.ObjectReference {
	l := []corev1.ObjectReference{}
	for ref := range r.existingResources {
		ref := ref
		l = append(l, ref)
	}
	return l
}
