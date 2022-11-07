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

package resource

import (
	"context"
	"fmt"

	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/utils/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// An APIFinalizer adds and removes finalizers to and from a resource.
type APIFinalizer struct {
	client    client.Client
	finalizer string
}

// NewAPIFinalizer returns a new APIFinalizer.
func NewAPIFinalizer(c client.Client, finalizer string) *APIFinalizer {
	return &APIFinalizer{client: c, finalizer: finalizer}
}

// AddFinalizer to the supplied Managed resource.
func (a *APIFinalizer) AddFinalizer(ctx context.Context, obj Object) error {
	if meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.AddFinalizer(obj, a.finalizer)
	return errors.Wrap(a.client.Update(ctx, obj), errUpdateObject)
}

// RemoveFinalizer from the supplied Managed resource.
func (a *APIFinalizer) RemoveFinalizer(ctx context.Context, obj Object) error {
	if !meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.RemoveFinalizer(obj, a.finalizer)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, obj)), errUpdateObject)
}

// AddFinalizer to the supplied Managed resource.
func (a *APIFinalizer) AddFinalizerString(ctx context.Context, obj Object, finalizerString string) error {
	//fmt.Printf("AddFinalizerString finalizerString: %s\n", finalizerString)
	f := obj.GetFinalizers()
	found := false
	for _, ff := range f {
		if ff == finalizerString {
			found = true
			return nil
		}
	}
	if !found {
		f = append(f, finalizerString)
		obj.SetFinalizers(f)
	}
	//fmt.Printf("AddFinalizerString finalizers end: %v\n", obj.GetFinalizers())
	return errors.Wrap(a.client.Update(ctx, obj), errUpdateObject)
}

// RemoveFinalizer from the supplied Managed resource.
func (a *APIFinalizer) RemoveFinalizerString(ctx context.Context, obj Object, finalizerString string) error {
	f := obj.GetFinalizers()
	fmt.Printf("RemoveFinalizerString finalizers start: %v\n", obj.GetFinalizers())
	for _, ff := range f {
		if ff == finalizerString {
			f = util.RemoveString(f, ff)
			obj.SetFinalizers(f)
		}
	}
	fmt.Printf("RemoveFinalizerString finalizers end: %v\n", obj.GetFinalizers())
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, obj)), errUpdateObject)
}

func (a *APIFinalizer) HasOtherFinalizer(ctx context.Context, obj Object) (bool, error) {
	for _, f := range obj.GetFinalizers() {
		if f != a.finalizer {
			return true, nil
		}
	}
	return false, nil
}

// A FinalizerFns satisfy the Finalizer interface.
type FinalizerFns struct {
	AddFinalizerFn          func(ctx context.Context, obj Object) error
	RemoveFinalizerFn       func(ctx context.Context, obj Object) error
	HasOtherFinalizerFn     func(ctx context.Context, obj Object) (bool, error)
	AddFinalizerStringFn    func(ctx context.Context, obj Object, finalizerString string) error
	RemoveFinalizerStringFn func(ctx context.Context, obj Object, finalizerString string) error
}

// AddFinalizer to the supplied resource.
func (f FinalizerFns) AddFinalizer(ctx context.Context, obj Object) error {
	return f.AddFinalizerFn(ctx, obj)
}

// RemoveFinalizer from the supplied resource.
func (f FinalizerFns) RemoveFinalizer(ctx context.Context, obj Object) error {
	return f.RemoveFinalizerFn(ctx, obj)
}

// RemoveFinalizer from the supplied resource.
func (f FinalizerFns) HasOtherFinalizer(ctx context.Context, obj Object) (bool, error) {
	return f.HasOtherFinalizerFn(ctx, obj)
}

// AddFinalizer to the supplied resource.
func (f FinalizerFns) AddFinalizerString(ctx context.Context, obj Object, finalizerString string) error {
	return f.AddFinalizerStringFn(ctx, obj, finalizerString)
}

// RemoveFinalizer from the supplied resource.
func (f FinalizerFns) RemoveFinalizerString(ctx context.Context, obj Object, finalizerString string) error {
	return f.RemoveFinalizerStringFn(ctx, obj, finalizerString)
}
