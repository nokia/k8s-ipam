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

	"github.com/go-logr/logr"
	"github.com/henderiw-k8s-lcnc/discovery/registrator"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/proxy/proxycache"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
    allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
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
	Registrator registrator.Registrator
	Group       string // Group of GVK for event handling
	Normalizefn Normalizefn
	ValidateFn  proxycache.RefreshRespValidatorFn
}

func New[T1, T2 client.Object](ctx context.Context, cfg Config) Proxy[T1, T2] {
	l := ctrl.Log.WithName(cfg.Name)

	pc := proxycache.New(&proxycache.Config{
		Registrator: cfg.Registrator,
	})
	cp := &clientproxy[T1, T2]{
		normalizeFn: cfg.Normalizefn,
		pc:          pc,
		l:           l,
	}
	// this is a helper to validate the response since the proxy is content unaware. It understands
	// KRM but nothing else
	pc.RegisterRefreshRespValidator(cfg.Group, cfg.ValidateFn)
	pc.Start(ctx)
	return cp
}

type clientproxy[T1, T2 client.Object] struct {
	normalizeFn Normalizefn
	pc          proxycache.ProxyCache
	//logger
	l logr.Logger
}

func (r *clientproxy[T1, T2]) GetProxyCache() proxycache.ProxyCache {
	return r.pc
}

func (r *clientproxy[T1, T2]) AddEventChs(ec map[schema.GroupVersionKind]chan event.GenericEvent) {
	r.pc.AddEventChs(ec)
}

func (r *clientproxy[T1, T2]) CreateIndex(ctx context.Context, cr T1) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	req := BuildAllocPb(cr, cr.GetName(), string(b), "never", meta.GetGVKFromObject(cr))
	return r.pc.CreateIndex(ctx, req)
}

func (r *clientproxy[T1, T2]) DeleteIndex(ctx context.Context, cr T1) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	req := BuildAllocPb(cr, cr.GetName(), string(b), "never", meta.GetGVKFromObject(cr))
	return r.pc.DeAllocate(ctx, req)
}

func (r *clientproxy[T1, T2]) GetAllocation(ctx context.Context, o client.Object, d any) (T2, error) {
	r.l.Info("get allocated prefix", "cr", o)
	var x T2
	// normalizes the input to the proxycache generalized allocation
	req, err := r.normalizeFn(o, d)
	if err != nil {
		return x, err
	}
	r.l.Info("get allocated prefix", "allobrequest", req)
	resp, err := r.pc.GetAllocation(ctx, req)
	if err != nil {
		return x, err
	}

	if err := json.Unmarshal([]byte(resp.Status), &x); err != nil {
		return x, err
	}
	r.l.Info("allocate prefix done", "result", x)
	return x, nil
}

func (r *clientproxy[T1, T2]) Allocate(ctx context.Context, o client.Object, d any) (T2, error) {
	r.l.Info("allocate prefix", "cr", o)
	var x T2
	// normalizes the input to the proxycache generalized allocation
	req, err := r.normalizeFn(o, d)
	if err != nil {
		return x, err
	}
	r.l.Info("allocate prefix", "allobrequest", req)

	resp, err := r.pc.Allocate(ctx, req)
	if err != nil {
		return x, err
	}
	if err := json.Unmarshal([]byte(resp.Status), &x); err != nil {
		return x, err
	}
	r.l.Info("allocate prefix done", "result", x)
	return x, nil
}

func (r *clientproxy[T1, T2]) DeAllocate(ctx context.Context, o client.Object, d any) error {
	// normalizes the input to the proxycache generalized allocation
	req, err := r.normalizeFn(o, d)
	if err != nil {
		return err
	}
	return r.pc.DeAllocate(ctx, req)
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
