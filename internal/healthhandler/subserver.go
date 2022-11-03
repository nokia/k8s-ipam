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
package healthhandler

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type SubServer interface {
	Check(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error)
	Watch(in *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error
}

func New() SubServer {
	s := &subServer{
		mu:        sync.RWMutex{},
		statusMap: map[string]healthpb.HealthCheckResponse_ServingStatus{"": healthpb.HealthCheckResponse_SERVING},
		updates:   make(map[string]map[healthpb.Health_WatchServer]chan healthpb.HealthCheckResponse_ServingStatus),
	}
	return s
}

type subServer struct {
	l         logr.Logger
	mu        sync.RWMutex
	statusMap map[string]healthpb.HealthCheckResponse_ServingStatus
	updates   map[string]map[healthpb.Health_WatchServer]chan healthpb.HealthCheckResponse_ServingStatus
}
