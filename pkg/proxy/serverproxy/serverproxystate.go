package serverproxy

import (
	"context"
	"sync"

	"github.com/hansthienpondt/nipam/pkg/table"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"google.golang.org/grpc/peer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ProxyState struct {
	m sync.RWMutex
	// key is clientName
	clients  map[string]*clientContext
	backends map[schema.GroupVersion]backend.Backend
}

type ProxyStateConfig struct {
	Backends map[schema.GroupVersion]backend.Backend
}

type clientContext struct {
	stream resourcepb.Resource_WatchClaimServer
	cancel context.CancelFunc
}

func NewProxyState(cfg *ProxyStateConfig) *ProxyState {
	return &ProxyState{
		clients: map[string]*clientContext{},
		//ipam:    c.Ipam,
		backends: cfg.Backends,
	}
}

func (r *ProxyState) AddCallBackFn(header *resourcepb.Header, stream resourcepb.Resource_WatchClaimServer) {
	p, _ := peer.FromContext(stream.Context())

	ownerGVK := meta.ResourcePbGVKTostring(*header.OwnerGvk)

	r.m.Lock()
	// cancelFn if a client adss another entry the client is misbehaving
	if clientCtx, ok := r.clients[p.Addr.String()+":::"+ownerGVK]; ok {
		clientCtx.cancel()
	}
	ctx, cancel := context.WithCancel(stream.Context())
	log := log.FromContext(ctx)

	r.clients[p.Addr.String()+":::"+ownerGVK] = &clientContext{
		stream: stream,
		cancel: cancel,
	}
	r.m.Unlock()

	// we already validated the existance of the backend before calling this function
	gv := meta.GetSchemaGVKFromResourcePbGVK(header.Gvk).GroupVersion()
	be := r.backends[gv]
	be.AddWatch(resourcev1alpha1.NephioOwnerGvkKey, ownerGVK, r.CreateCallBackFn(stream))

	for range ctx.Done() {
		r.DeleteCallBackFn(p.Addr.String(), ownerGVK, gv)
		log.Info("watch stopped", "ownerGvk", ownerGVK, "groupVersion", gv)
		return
	}
}

func (r *ProxyState) DeleteCallBackFn(clientName, ownerGvk string, gv schema.GroupVersion) {
	r.m.Lock()
	defer r.m.Unlock()
	r.backends[gv].DeleteWatch(resourcev1alpha1.NephioOwnerGvkKey, ownerGvk)
	delete(r.clients, clientName+":::"+ownerGvk)
}

func (r *ProxyState) CreateCallBackFn(stream resourcepb.Resource_WatchClaimServer) backend.CallbackFn {
	log := log.FromContext(context.Background())
	return func(routes table.Routes, statusCode resourcepb.StatusCode) {
		for _, route := range routes {
			if err := stream.Send(&resourcepb.WatchResponse{
				Header: &resourcepb.Header{
					Gvk: meta.PointerResourcePBGVK(meta.StringToResourcePbGVK(route.Labels()[resourcev1alpha1.NephioGvkKey])),
					Nsn: &resourcepb.NSN{
						Namespace: route.Labels()[resourcev1alpha1.NephioNsnNamespaceKey],
						Name:      route.Labels()[resourcev1alpha1.NephioNsnNameKey],
					},
					OwnerGvk: meta.PointerResourcePBGVK(meta.StringToResourcePbGVK(route.Labels()[resourcev1alpha1.NephioOwnerGvkKey])),
					OwnerNsn: &resourcepb.NSN{
						Namespace: route.Labels()[resourcev1alpha1.NephioOwnerNsnNamespaceKey],
						Name:      route.Labels()[resourcev1alpha1.NephioOwnerNsnNameKey],
					},
				},
				StatusCode: statusCode,
			}); err != nil {
				p, _ := peer.FromContext(stream.Context())
				addr := "unknown"
				if p != nil {
					addr = p.Addr.String()
				}
				log.Error(err, "callback failed", "client", addr, "ownerGvk", route.Labels()[resourcev1alpha1.NephioOwnerGvkKey])
			}
		}
	}
}
