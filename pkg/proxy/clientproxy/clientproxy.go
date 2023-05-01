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
	"encoding/json"
	"sync"
	"time"

	"github.com/go-logr/logr"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/alloc"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type Proxy[T1, T2 client.Object] interface {
	AddEventChs(map[schema.GroupVersionKind]chan event.GenericEvent)
	// Create creates the cache instance in the backend
	CreateIndex(ctx context.Context, cr T1) error
	// Delete deletes the cache instance in the backend
	DeleteIndex(ctx context.Context, cr T1) error
	// Get returns the allocated resource
	GetAllocation(ctx context.Context, cr client.Object, d any) (T2, error)
	// Allocate allocates a resource
	Allocate(ctx context.Context, cr client.Object, d any) (T2, error)
	// DeAllocate deallocates a resource
	DeAllocate(ctx context.Context, cr client.Object, d any) error
}

type Normalizefn func(o client.Object, d any) (*allocpb.AllocRequest, error)

type Config struct {
	Name        string
	Address     string
	Group       string // Group of GVK for event handling
	Normalizefn Normalizefn
	ValidateFn  RefreshRespValidatorFn
}

func New[T1, T2 client.Object](ctx context.Context, cfg Config) Proxy[T1, T2] {
	l := ctrl.Log.WithName(cfg.Name)

	cp := &clientproxy[T1, T2]{
		address:     cfg.Address,
		normalizeFn: cfg.Normalizefn,
		informer:    NewNopInformer(),
		cache:       NewCache(),
		validator:   NewResponseValidator(),
		l:           l,
	}
	// This is a helper to validate the response since the proxy is content unaware.
	// It understands KRM but not the details of the spec
	cp.validator.Add(cfg.Group, RefreshRespValidatorFn(cfg.ValidateFn))
	cp.start(ctx)
	return cp
}

type clientproxy[T1, T2 client.Object] struct {
	// adress for the server
	address string
	// client
	m           sync.RWMutex
	allocClient alloc.Client
	// watch channel for the watch
	watchCancel context.CancelFunc
	// normalizes the specific allocation to the AllocPB GRPC message
	normalizeFn Normalizefn
	// this is the cache with GVK namespace, name
	cache Cache
	// informer provides information through the generic event to the owner GVK
	informer Informer
	// validator
	validator ResponseValidator
	//logger
	l logr.Logger
}

// AddEventChs add the ownerGvk's event channels to the informer
func (r *clientproxy[T1, T2]) AddEventChs(eventChannels map[schema.GroupVersionKind]chan event.GenericEvent) {
	r.informer = NewInformer(eventChannels)
}

func (r *clientproxy[T1, T2]) start(ctx context.Context) {
	r.l.Info("starting...")
	// this is used to control the watch go routine
	watchCtx, cancel := context.WithCancel(ctx)
	r.watchCancel = cancel

	// the client connection would always be connected
	// we expect the connection to reconnect when there is a failure
	if err := r.createClient(watchCtx); err != nil {
		r.l.Error(err, "cannot create client")
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				// called when the controller gets cancelled
				// we cleanup the client
				r.deleteClient(watchCtx)
				return
			case <-time.After(time.Second * 5):
				// cache refresh handler
				// walks the cache and check the expiry time
				keysToRefresh := r.cache.ValidateExpiryTime(ctx)
				var wg sync.WaitGroup
				for objKey, allocpbResp := range keysToRefresh {
					r.l.Info("refresh allocation", "gvk", objKey.gvk, "nsn", objKey.nsn)
					wg.Add(1)
					t := time.Now().Add(time.Minute * 60)
					b, err := t.MarshalText()
					if err != nil {
						r.l.Error(err, "cannot marshal the time during refresh")
					}
					req := &allocpb.AllocRequest{
						Header:     allocpbResp.GetHeader(),
						Spec:       allocpbResp.GetSpec(),
						ExpiryTime: string(b)}

					ownerGvk := schema.GroupVersionKind{
						Group:   allocpbResp.GetHeader().GetOwnerGvk().GetGroup(),
						Version: allocpbResp.GetHeader().GetOwnerGvk().GetVersion(),
						Kind:    allocpbResp.GetHeader().GetOwnerGvk().GetKind(),
					}
					ownerNsn := types.NamespacedName{
						Namespace: allocpbResp.GetHeader().GetOwnerNsn().GetNamespace(),
						Name:      allocpbResp.GetHeader().GetOwnerNsn().GetName(),
					}
					group := allocpbResp.GetHeader().GetGvk().GetGroup()
					origresp := allocpbResp
					objKey := objKey

					go func() {
						defer wg.Done()

						// refresh the allocation
						resp, err := r.refreshAllocate(ctx, req)
						if err != nil {
							// if we get an error in the response, log it and inform the client
							r.l.Error(err, "refresh allocation failed")
							// remove the cache entry
							r.cache.Delete(objKey)
							r.informer.NotifyClient(ownerGvk, ownerNsn)
						}
						// TBD if we need more protection
						if resp != nil {
							r.l.Info("refresh resp", "resp", resp.Status)
							// Validate the response through the client proxy registered validator
							// if the validator is not happy with the response we notify the client
							if r.validator.Get(group) != nil {
								if !r.validator.Get(group)(origresp, resp) {
									r.l.Error(err, "refresh validation NOK")
									// remove the cache entry
									r.cache.Delete(objKey)
									r.informer.NotifyClient(ownerGvk, ownerNsn)
								}
							}
						}
					}()
				}
				wg.Wait()
			}
		}
	}()
}

func (r *clientproxy[T1, T2]) CreateIndex(ctx context.Context, cr T1) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	req := BuildAllocPb(cr, cr.GetName(), string(b), "never", meta.GetGVKFromObject(cr))
	allocClient, err := r.getClient()
	if err != nil {
		return err
	}
	_, err = allocClient.CreateIndex(ctx, req)
	return err
}

func (r *clientproxy[T1, T2]) refreshAllocate(ctx context.Context, alloc *allocpb.AllocRequest) (*allocpb.AllocResponse, error) {
	return r.allocate(ctx, alloc, true)
}

func (r *clientproxy[T1, T2]) DeleteIndex(ctx context.Context, cr T1) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	req := BuildAllocPb(cr, cr.GetName(), string(b), "never", meta.GetGVKFromObject(cr))
	allocClient, err := r.getClient()
	if err != nil {
		return err
	}
	_, err = allocClient.DeleteIndex(ctx, req)
	return err
}

func (r *clientproxy[T1, T2]) GetAllocation(ctx context.Context, o client.Object, d any) (T2, error) {
	r.l.Info("get allocated resource", "cr", o)
	var x T2
	// normalizes the input to the proxycache generalized allocation
	req, err := r.normalizeFn(o, d)
	if err != nil {
		return x, err
	}
	r.l.Info("get allocated resource", "allocPbRequest", req)
	allocClient, err := r.getClient()
	if err != nil {
		return x, err
	}
	// TBD if we need to use the cache here
	resp, err := allocClient.GetAllocation(ctx, req)
	if err != nil || resp.GetStatusCode() != allocpb.StatusCode_Valid {
		return x, err
	}

	if err := json.Unmarshal([]byte(resp.Status), &x); err != nil {
		return x, err
	}
	r.l.Info("allocate resource done", "result", x)
	return x, nil
}

func (r *clientproxy[T1, T2]) Allocate(ctx context.Context, o client.Object, d any) (T2, error) {
	r.l.Info("allocate resource", "cr", o)
	var x T2
	// normalizes the input to the proxycache generalized allocation
	req, err := r.normalizeFn(o, d)
	if err != nil {
		return x, err
	}
	r.l.Info("allocate resource", "allobrequest", req)

	resp, err := r.allocate(ctx, req, false)
	if err != nil {
		return x, err
	}
	if err := json.Unmarshal([]byte(resp.Status), &x); err != nil {
		return x, err
	}
	r.l.Info("allocate resource done", "result", x)
	return x, nil
}

// refresh flag indicates if the allocation is initiated for a refresh
func (r *clientproxy[T1, T2]) allocate(ctx context.Context, alloc *allocpb.AllocRequest, refresh bool) (*allocpb.AllocResponse, error) {
	allocClient, err := r.getClient()
	if err != nil {
		return nil, err
	}

	key := getKey(alloc)
	if !refresh {
		cacheData := r.cache.Get(key)
		if cacheData != nil {
			// check if the data is available and consistent
			if isCacheDataValid(cacheData, alloc) {
				r.l.Info("cache hit OK -> response from cache", "keyGVK", key.gvk, "keyNsn", key.nsn)
				return cacheData, nil
			}
		}
	}
	if refresh {
		r.l.Info("cache hit NOK -> refresh from backend server", "keyGVK", key.gvk, "keyNsn", key.nsn)
	} else {
		// allocate the resource from the central backend server
		r.l.Info("cache hit NOK -> response from backend server", "keyGVK", key.gvk, "keyNsn", key.nsn)
	}

	resp, err := allocClient.Allocate(ctx, alloc)
	if err != nil || resp.GetStatusCode() != allocpb.StatusCode_Valid {
		return resp, err
	}
	// if the allocation is successfull we add the entry in the cache
	r.cache.Add(key, resp)
	return resp, err
}

func (r *clientproxy[T1, T2]) DeAllocate(ctx context.Context, o client.Object, d any) error {
	// normalizes the input to the proxycache generalized allocation
	req, err := r.normalizeFn(o, d)
	if err != nil {
		return err
	}
	allocClient, err := r.getClient()
	if err != nil {
		return err
	}
	_, err = allocClient.DeAllocate(ctx, req)
	if err != nil {
		return err
	}
	// delete the cache only if the DeAllocation is successfull
	r.cache.Delete(getKey(req))
	return nil
}

func BuildAllocPb(o client.Object, nsnName, specBody, expiryTime string, gvk schema.GroupVersionKind) *allocpb.AllocRequest {
	ownerGVK := o.GetObjectKind().GroupVersionKind()
	// if the ownerGvk is in the labels we use this as ownerGVK
	ownerGVKValue, ok := o.GetLabels()[allocv1alpha1.NephioOwnerGvkKey]
	if ok {
		ownerGVK = meta.StringToGVK(ownerGVKValue)
	}
	return &allocpb.AllocRequest{
		Header: &allocpb.Header{
			Gvk: &allocpb.GVK{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind,
			},
			Nsn: &allocpb.NSN{
				Namespace: o.GetNamespace(),
				Name:      nsnName, // this will be overwritten for niInstance prefixes
			},
			OwnerGvk: &allocpb.GVK{
				Group:   ownerGVK.Group,
				Version: ownerGVK.Version,
				Kind:    ownerGVK.Kind,
			},
			OwnerNsn: &allocpb.NSN{
				Namespace: o.GetNamespace(),
				Name:      o.GetName(),
			},
		},
		Spec:       specBody,
		ExpiryTime: expiryTime,
	}
}
