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
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type GrpcServer struct {
	config Config
	allocpb.UnimplementedAllocationServer

	sem *semaphore.Weighted

	// logger
	l logr.Logger

	//Alloc Handlers
	allocHandler   AllocHandler
	deallocHandler DeAllocHandler

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

// Alloc Handlers
type AllocHandler func(context.Context, *allocpb.Request) (*allocpb.Response, error)

type DeAllocHandler func(context.Context, *allocpb.Request) (*allocpb.Response, error)

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

	allocpb.RegisterAllocationServer(grpcServer, s)
	s.l.Info("grpc server with allocation...")

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

func WithAllocHandler(h AllocHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.allocHandler = h
	}
}

func WithDeAllocHandler(h DeAllocHandler) func(*GrpcServer) {
	return func(s *GrpcServer) {
		s.deallocHandler = h
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
