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

package ipam

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type CallbackFn func(table.Routes, allocpb.StatusCode)

type updateContext struct {
	routes     []table.Route
	callBackFn backend.CallbackFn
}

type Watcher interface {
	addWatch(ownerGvkKey, ownerGvk string, fn backend.CallbackFn)
	deleteWatch(ownerGvkKey, ownerGvk string)
	handleUpdate(ctx context.Context, routes table.Routes, statusCode allocpb.StatusCode)
}

func newWatcher() Watcher {
	return &watcher{
		d: map[string]map[string]backend.CallbackFn{},
	}
}

type watcher struct {
	m sync.RWMutex
	// 1st key is ownerGvk key, 2nd key is ownerGVK
	d map[string]map[string]backend.CallbackFn
	l logr.Logger
}

func (r *watcher) addWatch(ownerGvkKey, ownerGvk string, fn backend.CallbackFn) {
	r.m.Lock()
	defer r.m.Unlock()

	if _, ok := r.d[ownerGvkKey]; !ok {
		r.d[ownerGvkKey] = map[string]backend.CallbackFn{}
	}
	r.d[ownerGvkKey][ownerGvk] = fn
}

func (r *watcher) deleteWatch(ownerGvkKey, ownerGvk string) {
	r.m.Lock()
	defer r.m.Unlock()

	if _, ok := r.d[ownerGvkKey]; ok {
		delete(r.d[ownerGvkKey], ownerGvk)
	}
	if len(r.d[ownerGvkKey]) == 0 {
		delete(r.d, ownerGvkKey)
	}
}

func (r *watcher) handleUpdate(ctx context.Context, routes table.Routes, statusCode allocpb.StatusCode) {
	r.l = log.FromContext(ctx)
	r.m.RLock()
	defer r.m.RUnlock()

	// build a new updatemap based on the values
	// we receive routes but we have to build a map based on ownerGVK Values
	updateMap := map[string]*updateContext{}

	// walk through all the routes
	// first check if the ownerGVK key is present
	// if so check the value and map them to the proper output map
	for _, route := range routes {
		for ownerGvkKey, values := range r.d {
			if ownerGvkValue, ok := route.Labels()[ownerGvkKey]; ok {
				for value, fn := range values {
					if ownerGvkValue == value {
						// initalize the updateMap if the value does not exist
						if _, ok := updateMap[ownerGvkValue]; !ok {
							updateMap[ownerGvkValue] = &updateContext{
								routes:     []table.Route{},
								callBackFn: fn,
							}
						}
						// add the routes that belong to this ownerGVK
						updateMap[ownerGvkValue].routes = append(updateMap[ownerGvkValue].routes, route)
					}
				}
			}
		}
	}

	// call the callback fn using the routes and the original status code
	for ownerGvk, updateContext := range updateMap {
		r.l.Info("watch event", "ownerGvk", ownerGvk, "Routes", updateContext.routes)
		updateContext.callBackFn(updateContext.routes, statusCode)
	}
}
