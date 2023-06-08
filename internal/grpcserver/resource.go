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

	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *GrpcServer) CreateIndex(ctx context.Context, req *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.createIndexHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) DeleteIndex(ctx context.Context, req *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.deleteIndexHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) GetClaim(ctx context.Context, req *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.getClaimHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) Claim(ctx context.Context, req *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.claimHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) DeleteClaim(ctx context.Context, req *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.sem.Release(1)
	resp, err := s.deleteClaimHandler(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *GrpcServer) WatchResource(in *resourcepb.WatchRequest, stream resourcepb.Resource_WatchClaimServer) error {
	err := s.acquireSem(stream.Context())
	if err != nil {
		return err
	}
	defer s.sem.Release(1)

	if s.watchHandler != nil {
		return s.watchClaimHandler(in, stream)
	}
	return status.Error(codes.Unimplemented, "")
}
