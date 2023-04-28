package serverproxy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"google.golang.org/grpc/peer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Proxy interface {
	CreateIndex(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.EmptyResponse, error)
	DeleteIndex(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.EmptyResponse, error)
	GetAllocation(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.AllocResponse, error)
	Allocate(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.AllocResponse, error)
	DeAllocate(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.EmptyResponse, error)
	Watch(in *allocpb.WatchRequest, stream allocpb.Allocation_WatchAllocServer) error
}

type Config struct {
	Backends map[schema.GroupVersion]backend.Backend
}

func New(cfg *Config) Proxy {
	l := ctrl.Log.WithName("server-proxy")
	return &serverproxy{
		backends:   cfg.Backends,
		proxyState: NewProxyState(&ProxyStateConfig{Backends: cfg.Backends}),
		l:          l,
	}
}

type serverproxy struct {
	backends   map[schema.GroupVersion]backend.Backend
	proxyState *ProxyState
	//logger
	l logr.Logger
}

func (r *serverproxy) CreateIndex(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.EmptyResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromAllocPbGVK(alloc.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk)
	}
	err := be.CreateIndex(ctx, []byte(alloc.Spec))
	if err != nil {
		r.l.Error(err, "cannot create index", "spec", alloc.Spec)
		return nil, err
	}
	r.l.Info("create index done")
	return &allocpb.EmptyResponse{}, nil
}

func (r *serverproxy) DeleteIndex(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.EmptyResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromAllocPbGVK(alloc.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk)
	}
	err := be.DeleteIndex(ctx, []byte(alloc.Spec))
	if err != nil {
		r.l.Error(err, "cannot delete index", "spec", alloc.Spec)
		return nil, err
	}
	r.l.Info("delete index done")
	return &allocpb.EmptyResponse{}, nil
}

func (r *serverproxy) GetAllocation(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.AllocResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromAllocPbGVK(alloc.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk)
	}
	b, err := be.GetAllocation(ctx, []byte(alloc.Spec))
	if err != nil {
		r.l.Error(err, "cannot get allocation", "spec", alloc.Spec)
		return nil, err
	}
	resp := &allocpb.AllocResponse{Header: alloc.Header, Spec: alloc.Spec, StatusCode: allocpb.StatusCode_Unknown, ExpiryTime: alloc.ExpiryTime}
	resp.Status = string(b)
	resp.StatusCode = allocpb.StatusCode_Valid
	r.l.Info("get allocation done")
	return resp, nil
}

func (r *serverproxy) Allocate(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.AllocResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromAllocPbGVK(alloc.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk)
	}

	b, err := be.Allocate(ctx, []byte(alloc.Spec))
	if err != nil {
		r.l.Error(err, "cannot allocate", "spec", alloc.Spec)
		return nil, err
	}
	resp := &allocpb.AllocResponse{Header: alloc.Header, Spec: alloc.Spec, StatusCode: allocpb.StatusCode_Unknown, ExpiryTime: alloc.ExpiryTime}
	resp.Status = string(b)
	resp.StatusCode = allocpb.StatusCode_Valid
	r.l.Info("allocation done")
	return resp, nil
}

func (r *serverproxy) DeAllocate(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.EmptyResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromAllocPbGVK(alloc.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", alloc.Header.Gvk)
	}
	err := be.DeAllocate(ctx, []byte(alloc.Spec))
	if err != nil {
		r.l.Error(err, "cannot deallocate", "spec", alloc.Spec)
		return nil, err
	}
	r.l.Info("deallocate done")
	return &allocpb.EmptyResponse{}, nil
}

func (r *serverproxy) Watch(in *allocpb.WatchRequest, stream allocpb.Allocation_WatchAllocServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)
	addr := "unknown"
	if p != nil {
		addr = p.Addr.String()
	}
	r.l.Info("watch started", "client", addr, "ownerGvk", in.Header.OwnerGvk)

	_, ok := r.backends[meta.GetSchemaGVKFromAllocPbGVK(in.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", in.Header.Gvk), "backendend not registered")
		return fmt.Errorf("backend not registered, got: %v", in.Header.Gvk)
	}

	r.proxyState.AddCallBackFn(in.Header, stream)
	return nil
}
