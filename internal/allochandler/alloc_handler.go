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

	"github.com/nokia/k8s-ipam/internal/ipam"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (s *subServer) Allocation(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	s.l = log.FromContext(ctx)
	s.l.Info("allocate", "alloc", alloc)

	prefix, err := s.ipam.AllocateIPPrefix(ctx, ipam.BuildAllocationFromGRPCAlloc(alloc))
	if err != nil {
		return nil, err
	}
	return &allocpb.Response{
		AllocatedPrefix: prefix.AllocatedPrefix,
		Gateway:         prefix.Gateway,
	}, nil
}

func (s *subServer) DeAllocation(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	s.l = log.FromContext(ctx)
	s.l.Info("deallocate", "alloc", alloc)

	//allocs := []*ipamv1alpha1.IPAllocation{}
	//allocs = append(allocs, buildAlloc(alloc))
	if err := s.ipam.DeAllocateIPPrefix(ctx, ipam.BuildAllocationFromGRPCAlloc(alloc)); err != nil {
		return nil, err
	}
	return &allocpb.Response{}, nil
}
