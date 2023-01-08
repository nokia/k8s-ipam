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
	"sync"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
)

func WithRoute(group string, handler AlloHandler) func(SubServer) {
	return func(s SubServer) {
		s.WithRoute(group, handler)
	}
}

type AlloHandler interface {
	Allocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error)
	DeAllocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.EmptyResponse, error)
}

type Option func(SubServer)

type SubServer interface {
	WithRoute(group string, handler AlloHandler)
	Allocate(context.Context, *allocpb.Request) (*allocpb.Response, error)
	DeAllocate(context.Context, *allocpb.Request) (*allocpb.EmptyResponse, error)
}

func New(opts ...Option) SubServer {
	r := &subServer{
		h: map[string]AlloHandler{},
	}

	for _, o := range opts {
		o(r)
	}
	return r
}

type subServer struct {
	m sync.RWMutex
	h map[string]AlloHandler
	l logr.Logger
}

func (r *subServer) WithRoute(group string, handler AlloHandler) {
	r.h[group] = handler
}
