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
	"net"
	"sync"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type GrpcServer struct {
	config Config
	resourcepb.UnimplementedResourceServer

	sem *semaphore.Weighted

	// logger
	l logr.Logger

	//Resource Handlers
	createIndexHandler CreateIndexHandler
	deleteIndexHandler DeleteIndexHandler
	getClaimHandler    GetClaimHandler
	claimHandler       ClaimHandler
	deleteClaimHandler DeleteClaimHandler
	watchClaimHandler  WatchClaimHandler

	//health handlers
	checkHandler CheckHandler
	watchHandler WatchHandler
	//
	// cached certificate
	cm *sync.Mutex
}

// Health Handlers
type CheckHandler func(context.Context, *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error)

type WatchHandler func(*healthpb.HealthCheckRequest, healthpb.Health_WatchServer) error

// Resource Handlers
type CreateIndexHandler func(context.Context, *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error)

type DeleteIndexHandler func(context.Context, *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error)

type GetClaimHandler func(context.Context, *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error)

type ClaimHandler func(context.Context, *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error)

type DeleteClaimHandler func(context.Context, *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error)

type WatchClaimHandler func(*resourcepb.WatchRequest, resourcepb.Resource_WatchClaimServer) error

type Option func(*GrpcServer)

func New(c Config, opts ...Option) *GrpcServer {
	c.setDefaults()
	s := &GrpcServer{
		config: c,
		sem:    semaphore.NewWeighted(c.MaxRPC),
		cm:     &sync.Mutex{},
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

func (s *GrpcServer) Start(ctx context.Context) error {
	s.l = log.FromContext(ctx)
	s.l.Info("grpc server start...")
	s.l.Info("grpc server start",
		"address", s.config.Address,
		"certDir", s.config.CertDir,
		"certName", s.config.CertName,
		"keyName", s.config.KeyName,
		"caName", s.config.CaName,
	)
	l, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return errors.Wrap(err, "cannot listen")
	}
	opts, err := s.serverOpts(ctx)
	if err != nil {
		return err
	}
	// create a gRPC server object
	grpcServer := grpc.NewServer(opts...)

	resourcepb.RegisterResourceServer(grpcServer, s)
	s.l.Info("grpc server with resource...")

	healthpb.RegisterHealthServer(grpcServer, s)
	s.l.Info("grpc server with health...")

	s.l.Info("starting grpc server...")
	err = grpcServer.Serve(l)
	if err != nil {
		s.l.Info("gRPC serve failed", "error", err)
		return err
	}
	return nil
}

func WithCheckHandler(h CheckHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.checkHandler = h
	}
}

func WithWatchHandler(h WatchHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.watchHandler = h
	}
}

func WithCreateIndexHandler(h CreateIndexHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.createIndexHandler = h
	}
}

func WithDeleteIndexHandler(h DeleteIndexHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.deleteIndexHandler = h
	}
}

func WithGetClaimHandler(h GetClaimHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.getClaimHandler = h
	}
}

func WithClaimHandler(h ClaimHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.claimHandler = h
	}
}

func WithDeleteClaimHandler(h DeleteClaimHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.deleteClaimHandler = h
	}
}

func WithWatchClaimHandler(h WatchClaimHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.watchClaimHandler = h
	}
}

func (s *GrpcServer) acquireSem(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return s.sem.Acquire(ctx, 1)
	}
}
