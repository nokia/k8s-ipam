package serverproxy

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"google.golang.org/grpc/peer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ProxyState struct {
	m sync.RWMutex
	// key is clientName
	clients  map[string]*clientContext
	backends map[schema.GroupVersion]backend.Backend
	l        logr.Logger
}

type ProxyStateConfig struct {
	Backends map[schema.GroupVersion]backend.Backend
}

type clientContext struct {
	stream allocpb.Allocation_WatchAllocServer
	cancel context.CancelFunc
}

func NewProxyState(cfg *ProxyStateConfig) *ProxyState {
	l := ctrl.Log.WithName("server-proxy-state")
	return &ProxyState{
		clients: map[string]*clientContext{},
		//ipam:    c.Ipam,
		backends: cfg.Backends,
		l:        l,
	}
}

func (r *ProxyState) AddCallBackFn(header *allocpb.Header, stream allocpb.Allocation_WatchAllocServer) {
	p, _ := peer.FromContext(stream.Context())

	ownerGVK := meta.AllocPbGVKTostring(*header.OwnerGvk)

	r.m.Lock()
	// cancelFn if a client adss another entry the client is misbehaving
	if clientCtx, ok := r.clients[p.Addr.String()+":::"+ownerGVK]; ok {
		clientCtx.cancel()
	}
	ctx, cancel := context.WithCancel(stream.Context())

	r.clients[p.Addr.String()+":::"+ownerGVK] = &clientContext{
		stream: stream,
		cancel: cancel,
	}
	r.m.Unlock()

	// we already validated the existance of the backend before calling this function
	gv := meta.GetSchemaGVKFromAllocPbGVK(header.Gvk).GroupVersion()
	be := r.backends[gv]
	be.AddWatch(allocv1alpha1.NephioOwnerGvkKey, ownerGVK, r.CreateCallBackFn(stream))

	for range ctx.Done() {
		r.DeleteCallBackFn(p.Addr.String(), ownerGVK, gv)
		r.l.Info("watch stopped", "ownerGvk", ownerGVK, "groupVersion", gv)
		return

	}
}

func (r *ProxyState) DeleteCallBackFn(clientName, ownerGvk string, gv schema.GroupVersion) {
	r.m.Lock()
	defer r.m.Unlock()
	r.backends[gv].DeleteWatch(allocv1alpha1.NephioOwnerGvkKey, ownerGvk)
	delete(r.clients, clientName+":::"+ownerGvk)
}

func (r *ProxyState) CreateCallBackFn(stream allocpb.Allocation_WatchAllocServer) backend.CallbackFn {
	return func(routes table.Routes, statusCode allocpb.StatusCode) {
		for _, route := range routes {
			if err := stream.Send(&allocpb.WatchResponse{
				Header: &allocpb.Header{
					Gvk: meta.PointerAllocPBGVK(meta.StringToAllocPbGVK(route.Labels()[allocv1alpha1.NephioGvkKey])),
					Nsn: &allocpb.NSN{
						Namespace: route.Labels()[allocv1alpha1.NephioNsnNamespaceKey],
						Name:      route.Labels()[allocv1alpha1.NephioNsnNameKey],
					},
					OwnerGvk: meta.PointerAllocPBGVK(meta.StringToAllocPbGVK(route.Labels()[allocv1alpha1.NephioOwnerGvkKey])),
					OwnerNsn: &allocpb.NSN{
						Namespace: route.Labels()[allocv1alpha1.NephioOwnerNsnNamespaceKey],
						Name:      route.Labels()[allocv1alpha1.NephioOwnerNsnNameKey],
					},
				},
				StatusCode: statusCode,
			}); err != nil {
				p, _ := peer.FromContext(stream.Context())
				addr := "unknown"
				if p != nil {
					addr = p.Addr.String()
				}
				r.l.Error(err, "callback failed", "client", addr, "ownerGvk", route.Labels()[allocv1alpha1.NephioOwnerGvkKey])
			}
		}
	}
}
