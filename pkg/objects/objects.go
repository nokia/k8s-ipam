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

package objects

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExtObjects interface {
	client.ObjectList

	// GetItems returns the list of managed resources.
	GetItems() []client.Object
}

type Objects struct {
	ExtObjects
}

func (r Objects) iterator() *iterator[client.Object] {
	return &iterator[client.Object]{curIdx: -1, items: r.GetItems()}
}

func (r Objects) GetAllObjects() []client.Object {
	objs := []client.Object{}

	iter := r.iterator()
	for iter.HasNext() {
		objs = append(objs, iter.Value())
	}
	return objs
}

func (r Objects) GetSelectedObjects(s *metav1.LabelSelector) ([]client.Object, error) {
	selector, err := metav1.LabelSelectorAsSelector(s)
	if err != nil {
		return nil, err
	}

	objs := []client.Object{}
	uniqueValues := map[string]struct{}{}

	iter := r.iterator()
	for iter.HasNext() {
		v := iter.Value()
		if selector.Matches(labels.Set(v.GetLabels())) {
			name := types.NamespacedName{Name: v.GetName(), Namespace: v.GetNamespace()}
			if _, ok := uniqueValues[name.String()]; !ok {
				objs = append(objs, iter.Value())
			}
			uniqueValues[name.String()] = struct{}{}
		}
	}
	return objs, nil
}
