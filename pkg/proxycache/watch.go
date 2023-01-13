package proxycache

import (
	"context"
	"errors"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *proxycache) startWatches(ctx context.Context) {
	r.l = log.FromContext(ctx)
	// subscribe to the server for events
	go func() {
		defer r.stopWatches()
		for _, gvk := range r.informer.GetGVK() {
			go r.startWatch(ctx, gvk)
		}

		for range ctx.Done() {
			r.l.Info("watch stopped")
			return
		}
	}()
}

func (r *proxycache) stopWatches() {
	r.watchCancel()
}

func (r *proxycache) startWatch(ctx context.Context, gvk schema.GroupVersionKind) {
	// client side of the RPC stream
	var stream allocpb.Allocation_WatchAllocClient

	for {
		allocClient, err := r.getClient()
		if err != nil {
			r.l.Error(err, "failed to get client")
			continue
		}

		if stream == nil {
			if stream, err = allocClient.WatchAlloc(ctx, &allocpb.WatchRequest{
				Header: &allocpb.Header{
					OwnerGvk: meta.GetAllocPbGVKFromSchemaGVK(gvk),
					Gvk:      meta.GetAllocPbGVKFromSchemaGVK(ipamv1alpha1.IPAllocationGroupVersionKind), // TODO make it independent
				}}); err != nil && !errors.Is(err, context.Canceled) {
				if er, ok := status.FromError(err); ok {
					switch er.Code() {
					case codes.Canceled:
						// dont log when context got cancelled
					default:
						r.l.Error(err, "failed to subscribe")
					}
				}
				//time.Sleep(time.Second * 1) //- resilience for server crash
				// retry on failure
				continue
			}
		}
		response, err := stream.Recv()
		if err != nil && !errors.Is(err, context.Canceled) {
			if er, ok := status.FromError(err); ok {
				switch er.Code() {
				case codes.Canceled:
					// dont log when context got cancelled
				default:
					r.l.Error(err, "failed to receive a message from stream")
				}
			}
			// clearing the stream will force the client to resubscribe in the next iteration
			stream = nil
			//time.Sleep(time.Second * 1) //- resilience for server crash
			// retry on failure
			continue
		}
		r.l.Info("watch response -> notify client", "gvk", gvk, "header", response.Header, "state", response.StatusCode)
		if response.StatusCode == allocpb.StatusCode_Unknown {
			// invalidate the cache
			r.cache.Delete(ObjectKindKey{
				gvk: meta.GetSchemaGVKFromAllocPbGVK(response.Header.Gvk),
				nsn: meta.GetTypeNSNFromAllocPbNSN(response.Header.Nsn),
			})
		}

		// inform the ownerGVK to retrigger a reconcilation event
		r.informer.NotifyClient(
			meta.GetSchemaGVKFromAllocPbGVK(response.Header.OwnerGvk),
			meta.GetTypeNSNFromAllocPbNSN(response.Header.OwnerNsn),
		)
	}
}
