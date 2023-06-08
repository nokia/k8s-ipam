/*
Copyright 2023 The Nephio Authors.

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

package clientproxy

import (
	"context"
	"errors"
	"time"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *clientproxy[T1, T2]) startWatches(ctx context.Context) {
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

func (r *clientproxy[T1, T2]) stopWatches() {
	r.watchCancel()
}

func (r *clientproxy[T1, T2]) startWatch(ctx context.Context, gvk schema.GroupVersionKind) {
	// client side of the RPC stream
	var stream resourcepb.Resource_WatchClaimClient

	for {
		resourceClient, err := r.getClient()
		if err != nil {
			r.l.Error(err, "failed to get client")
			continue
		}

		if stream == nil {
			if stream, err = resourceClient.WatchClaim(ctx, &resourcepb.WatchRequest{
				Header: &resourcepb.Header{
					OwnerGvk: meta.PointerResourcePBGVK(meta.GetResourcePbGVKFromSchemaGVK(gvk)),
					Gvk:      meta.PointerResourcePBGVK(meta.GetResourcePbGVKFromSchemaGVK(ipamv1alpha1.IPClaimGroupVersionKind)), // TODO make it independent
				}}); err != nil && !errors.Is(err, context.Canceled) {
				if er, ok := status.FromError(err); ok {
					switch er.Code() {
					case codes.Canceled:
						// dont log when context got cancelled
					default:
						r.l.Error(err, "failed to subscribe")
					}
				}
				time.Sleep(time.Second * 1) //- resilience for server crash
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
			time.Sleep(time.Second * 1) //- resilience for server crash
			// retry on failure
			continue
		}
		r.l.Info("watch response -> notify client", "gvk", gvk, "header", response.Header, "state", response.StatusCode)
		if response.StatusCode == resourcepb.StatusCode_Unknown {
			// invalidate the cache
			r.cache.Delete(ObjectKindKey{
				gvk: meta.GetSchemaGVKFromResourcePbGVK(response.Header.Gvk),
				nsn: meta.GetTypeNSNFromResourcePbNSN(response.Header.Nsn),
			})
		}

		// inform the ownerGVK to retrigger a reconcilation event
		r.informer.NotifyClient(
			meta.GetSchemaGVKFromResourcePbGVK(response.Header.OwnerGvk),
			meta.GetTypeNSNFromResourcePbNSN(response.Header.OwnerNsn),
		)
	}
}
