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

package allochandler

import (
	"context"
	"fmt"

	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *subServer) Allocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("allocate", "alloc", alloc)

	h :=  r.getHandler(alloc.Header.Gvk.Group) 
	if h == nil {
		return nil, fmt.Errorf("unregistered route, route error, got %v", alloc.Header)
	}
	return h.Allocate(ctx, alloc)

}

func (r *subServer) DeAllocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.EmptyResponse, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("deallocate", "alloc", alloc)

	h := r.getHandler(alloc.Header.Gvk.Group) 
	if h == nil {
		return nil, fmt.Errorf("unregistered route, route error, got %v", alloc.Header)
	}
	return h.DeAllocate(ctx, alloc)
}

func (r *subServer) getHandler(group string) AlloHandler {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.h[group]
}
