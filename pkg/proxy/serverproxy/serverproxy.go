package serverproxy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"google.golang.org/grpc/peer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Proxy interface {
	CreateIndex(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error)
	DeleteIndex(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error)
	GetClaim(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error)
	Claim(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error)
	DeleteClaim(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error)
	Watch(in *resourcepb.WatchRequest, stream resourcepb.Resource_WatchClaimServer) error
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

func (r *serverproxy) CreateIndex(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromResourcePbGVK(claim.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk)
	}
	err := be.CreateIndex(ctx, []byte(claim.Spec))
	if err != nil {
		r.l.Error(err, "cannot create index", "spec", claim.Spec)
		return nil, err
	}
	r.l.Info("create index done")
	return &resourcepb.EmptyResponse{}, nil
}

func (r *serverproxy) DeleteIndex(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromResourcePbGVK(claim.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk)
	}
	err := be.DeleteIndex(ctx, []byte(claim.Spec))
	if err != nil {
		r.l.Error(err, "cannot delete index", "spec", claim.Spec)
		return nil, err
	}
	r.l.Info("delete index done")
	return &resourcepb.EmptyResponse{}, nil
}

func (r *serverproxy) GetClaim(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromResourcePbGVK(claim.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk)
	}
	b, err := be.GetClaim(ctx, []byte(claim.Spec))
	if err != nil {
		r.l.Error(err, "cannot get claim", "spec", claim.Spec)
		return nil, err
	}
	resp := &resourcepb.ClaimResponse{Header: claim.Header, Spec: claim.Spec, StatusCode: resourcepb.StatusCode_Unknown, ExpiryTime: claim.ExpiryTime}
	resp.Status = string(b)
	resp.StatusCode = resourcepb.StatusCode_Valid
	r.l.Info("get claim done")
	return resp, nil
}

func (r *serverproxy) Claim(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.ClaimResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromResourcePbGVK(claim.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk)
	}

	b, err := be.Claim(ctx, []byte(claim.Spec))
	if err != nil {
		r.l.Error(err, "cannot claim", "spec", claim.Spec)
		return nil, err
	}
	resp := &resourcepb.ClaimResponse{Header: claim.Header, Spec: claim.Spec, StatusCode: resourcepb.StatusCode_Unknown, ExpiryTime: claim.ExpiryTime}
	resp.Status = string(b)
	resp.StatusCode = resourcepb.StatusCode_Valid
	r.l.Info("claim done")
	return resp, nil
}

func (r *serverproxy) DeleteClaim(ctx context.Context, claim *resourcepb.ClaimRequest) (*resourcepb.EmptyResponse, error) {
	be, ok := r.backends[meta.GetSchemaGVKFromResourcePbGVK(claim.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk), "backendend not registered")
		return nil, fmt.Errorf("backend not registered, got: %v", claim.Header.Gvk)
	}
	err := be.DeleteClaim(ctx, []byte(claim.Spec))
	if err != nil {
		r.l.Error(err, "cannot delete claim", "spec", claim.Spec)
		return nil, err
	}
	r.l.Info("delete claim done")
	return &resourcepb.EmptyResponse{}, nil
}

func (r *serverproxy) Watch(in *resourcepb.WatchRequest, stream resourcepb.Resource_WatchClaimServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)
	addr := "unknown"
	if p != nil {
		addr = p.Addr.String()
	}
	r.l.Info("watch started", "client", addr, "ownerGvk", in.Header.OwnerGvk)

	_, ok := r.backends[meta.GetSchemaGVKFromResourcePbGVK(in.Header.Gvk).GroupVersion()]
	if !ok {
		r.l.Error(fmt.Errorf("backend not registered, got: %v", in.Header.Gvk), "backendend not registered")
		return fmt.Errorf("backend not registered, got: %v", in.Header.Gvk)
	}

	r.proxyState.AddCallBackFn(in.Header, stream)
	return nil
}
