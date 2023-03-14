package serveripamproxy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/ipam"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"google.golang.org/grpc/peer"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Proxy interface {
	Allocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error)
	DeAllocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.EmptyResponse, error)
	Watch(in *allocpb.WatchRequest, stream allocpb.Allocation_WatchAllocServer) error
}

type Config struct {
	Ipam ipam.Ipam
}

func New(c *Config) Proxy {
	l := ctrl.Log.WithName("ipam-server-proxy")
	return &serverproxy{
		ipam:       c.Ipam,
		proxyState: NewProxyState(&ProxyStateConfig{Ipam: c.Ipam}),
		l:          l,
	}
}

type serverproxy struct {
	ipam       ipam.Ipam
	proxyState *ProxyState
	//logger
	l logr.Logger
}

func (r *serverproxy) Allocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	allocResp := &allocpb.Response{Header: alloc.Header, Spec: alloc.Spec, StatusCode: allocpb.StatusCode_Unknown, ExpiryTime: alloc.ExpiryTime}
	switch alloc.Header.Gvk.Kind {
	case ipamv1alpha1.NetworkInstanceKind:
		r.l.Info("create ipam instance", "kind", alloc.Header.Gvk.Kind)
		// this is a create of the network instance
		cr := &ipamv1alpha1.NetworkInstance{}
		if err := json.Unmarshal([]byte(alloc.Spec), cr); err != nil {
			return allocResp, err
		}
		if err := r.ipam.Create(ctx, cr); err != nil {
			return allocResp, err
		}
		allocResp.Status = ""
		allocResp.StatusCode = allocpb.StatusCode_Valid
		return allocResp, nil
	case ipamv1alpha1.IPAllocationKind:
		r.l.Info("allocate prefix", "kind", alloc.Header.Gvk.Kind)
		// this is an ip allocation
		cr := &ipamv1alpha1.IPAllocation{}
		if err := json.Unmarshal([]byte(alloc.Spec), cr); err != nil {
			return allocResp, err
		}
		ipallocStatus, err := r.ipam.AllocateIPPrefix(ctx, cr)
		if err != nil {
			return allocResp, err
		}
		b, err := json.Marshal(ipallocStatus)
		if err != nil {
			return allocResp, err
		}
		allocResp.Status = string(b)
		allocResp.StatusCode = allocpb.StatusCode_Valid
		r.l.Info("allocate prefix done", "status", string(b))
		return allocResp, nil
	default:
		r.l.Info("unexpected kind in allocate", "kind", alloc.Header.Gvk.Kind)
		return allocResp, fmt.Errorf("unexpected kind in ipam server proxy, got: %s", alloc.Header.Gvk.Kind)
	}
}

func (r *serverproxy) DeAllocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.EmptyResponse, error) {
	switch alloc.Header.Gvk.Kind {
	case ipamv1alpha1.NetworkInstanceKind:
		r.l.Info("delete ipam instance", "kind", alloc.Header.Gvk.Kind)
		// this is a delete of the network instance
		cr := &ipamv1alpha1.NetworkInstance{}
		if err := json.Unmarshal([]byte(alloc.Spec), cr); err != nil {
			return &allocpb.EmptyResponse{}, err
		}
		r.ipam.Delete(ctx, cr)
		return &allocpb.EmptyResponse{}, nil
	case ipamv1alpha1.IPAllocationKind:
		r.l.Info("deallocate prefix", "kind", alloc.Header.Gvk.Kind)
		// this is an ip deallocation
		// this is an ip allocation
		cr := &ipamv1alpha1.IPAllocation{}
		if err := json.Unmarshal([]byte(alloc.Spec), cr); err != nil {
			return nil, err
		}
		return &allocpb.EmptyResponse{}, r.ipam.DeAllocateIPPrefix(ctx, cr)
	default:
		r.l.Info("unexpected kind in allocate", "kind", alloc.Header.Gvk.Kind)
		return nil, fmt.Errorf("unexpected kind in ipam server proxy, got: %s", alloc.Header.Gvk.Kind)
	}
}

func (r *serverproxy) Watch(in *allocpb.WatchRequest, stream allocpb.Allocation_WatchAllocServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)
	addr := "unknown"
	if p != nil {
		addr = p.Addr.String()
	}
	r.l.Info("watch started", "client", addr, "ownerGvk", in.Header.OwnerGvk)

	r.proxyState.AddCallBackFn(meta.AllocPbGVKTostring(in.Header.OwnerGvk), stream)
	return nil
}
