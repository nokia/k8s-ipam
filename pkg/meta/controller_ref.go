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

package meta

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TypedReferenceTo(o metav1.Object, of schema.GroupVersionKind) *corev1.ObjectReference {
	v, k := of.ToAPIVersionAndKind()
	return &corev1.ObjectReference{
		APIVersion: v,
		Kind:       k,
		Name:       o.GetName(),
		UID:        o.GetUID(),
	}
}

// AsOwner converts the supplied object reference to an owner reference.
func AsOwner(r *corev1.ObjectReference) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Name:       r.Name,
		UID:        r.UID,
	}
}

// AsController converts the supplied object reference to a controller
// reference. You may also consider using metav1.NewControllerRef.
func AsController(r *corev1.ObjectReference) metav1.OwnerReference {
	c := true
	ref := AsOwner(r)
	ref.Controller = &c
	return ref
}

// HaveSameController returns true if both supplied objects are controlled by
// the same object.
func HaveSameController(a, b metav1.Object) bool {
	ac := metav1.GetControllerOf(a)
	bc := metav1.GetControllerOf(b)

	// We do not consider two objects without any controller to have
	// the same controller.
	if ac == nil || bc == nil {
		return false
	}

	return ac.UID == bc.UID
}

// AddOwnerReference to the supplied object' metadata. Any existing owner with
// the same UID as the supplied reference will be replaced.
func AddOwnerReference(o metav1.Object, r metav1.OwnerReference) {
	refs := o.GetOwnerReferences()
	for i := range refs {
		if refs[i].UID == r.UID {
			refs[i] = r
			o.SetOwnerReferences(refs)
			return
		}
	}
	o.SetOwnerReferences(append(refs, r))
}

// AddControllerReference to the supplied object's metadata. Any existing owner
// with the same UID as the supplied reference will be replaced. Returns an
// error if the supplied object is already controlled by a different owner.
func AddControllerReference(o metav1.Object, r metav1.OwnerReference) error {
	if c := metav1.GetControllerOf(o); c != nil && c.UID != r.UID {
		return errors.Errorf("%s is already controlled by %s %s (UID %s)", o.GetName(), c.Kind, c.Name, c.UID)
	}

	AddOwnerReference(o, r)
	return nil
}
