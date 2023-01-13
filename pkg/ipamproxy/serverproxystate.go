package ipamproxy

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/ipam"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"google.golang.org/grpc/peer"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ProxyState struct {
	// key is clientName
	m       sync.RWMutex
	clients map[string]*clientContext
	ipam    ipam.Ipam
	l       logr.Logger
}

type ProxyStateConfig struct {
	Ipam ipam.Ipam
}

type clientContext struct {
	stream allocpb.Allocation_WatchAllocServer
	cancel context.CancelFunc
}

func NewProxyState(c *ProxyStateConfig) *ProxyState {
	l := ctrl.Log.WithName("ipam-server-proxy-state")
	return &ProxyState{
		clients: map[string]*clientContext{},
		ipam:    c.Ipam,
		l:       l,
	}
}

func (r *ProxyState) AddCallBackFn(ownerGvk string, stream allocpb.Allocation_WatchAllocServer) {
	p, _ := peer.FromContext(stream.Context())

	r.m.Lock()
	// cancelFn if a client adss another entry the client is misbehaving
	if clientCtx, ok := r.clients[p.Addr.String()+":::"+ownerGvk]; ok {
		clientCtx.cancel()
	}
	ctx, cancel := context.WithCancel(stream.Context())

	r.clients[p.Addr.String()+":::"+ownerGvk] = &clientContext{
		stream: stream,
		cancel: cancel,
	}
	r.m.Unlock()

	r.ipam.AddWatch(ipamv1alpha1.NephioOwnerGvkKey, ownerGvk, r.CreateCallBackFn(stream))

	for range ctx.Done() {
		r.DeleteCallBackFn(p.Addr.String(), ownerGvk)
		r.l.Info("watch stopped", "ownerGvk", ownerGvk)
		return

	}
}

func (r *ProxyState) DeleteCallBackFn(clientName, ownerGvk string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.ipam.DeleteWatch(ipamv1alpha1.NephioOwnerGvkKey, ownerGvk)
	delete(r.clients, clientName+":::"+ownerGvk)
}

func (r *ProxyState) CreateCallBackFn(stream allocpb.Allocation_WatchAllocServer) ipam.CallbackFn {
	return func(routes table.Routes, statusCode allocpb.StatusCode) {
		for _, route := range routes {
			if err := stream.Send(&allocpb.WatchResponse{
				Header: &allocpb.Header{
					Gvk:      meta.StringToAllocPbGVK(route.Labels()[ipamv1alpha1.NephioGvkKey]),
					Nsn:      meta.StringToAllocPbNsn(route.Labels()[ipamv1alpha1.NephioNsnKey]),
					OwnerGvk: meta.StringToAllocPbGVK(route.Labels()[ipamv1alpha1.NephioOwnerGvkKey]),
					OwnerNsn: meta.StringToAllocPbNsn(route.Labels()[ipamv1alpha1.NephioOwnerNsnKey]),
				},
				StatusCode: statusCode,
			}); err != nil {
				p, _ := peer.FromContext(stream.Context())
				addr := "unknown"
				if p != nil {
					addr = p.Addr.String()
				}
				r.l.Error(err, "callback failed", "client", addr, "ownerGvk", route.Labels()[ipamv1alpha1.NephioOwnerGvkKey])
			}
		}
	}
}
