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

package grpcserver

import (
	"context"

	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *GrpcServer) Get(ctx context.Context, req *allocpb.Request) (*allocpb.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.allocGetHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) Allocate(ctx context.Context, req *allocpb.Request) (*allocpb.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.allocHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) DeAllocate(ctx context.Context, req *allocpb.Request) (*allocpb.EmptyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.deallocHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) WatchAlloc(in *allocpb.WatchRequest, stream allocpb.Allocation_WatchAllocServer) error {
	err := s.acquireSem(stream.Context())
	if err != nil {
		return err
	}
	defer s.sem.Release(1)

	if s.watchHandler != nil {
		return s.watchAllocHandler(in, stream)
	}
	return status.Error(codes.Unimplemented, "")
}
