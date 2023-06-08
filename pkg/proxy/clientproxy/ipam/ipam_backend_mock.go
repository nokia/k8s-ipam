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

package ipam

import (
	"context"
	"encoding/json"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func NewBackendMock(be backend.Backend) clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim] {
	return &bemock{
		be: be,
	}
}

type bemock struct {
	be backend.Backend
}

func (r *bemock) AddEventChs(map[schema.GroupVersionKind]chan event.GenericEvent) {}

func (r *bemock) CreateIndex(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	return r.be.CreateIndex(ctx, b)
}

func (r *bemock) DeleteIndex(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	return r.be.DeleteIndex(ctx, b)
}

func (r *bemock) GetClaim(ctx context.Context, cr client.Object, d any) (*ipamv1alpha1.IPClaim, error) {
	b, err := NormalizeKRMToBytes(cr, d)
	if err != nil {
		return nil, err
	}
	b, err = r.be.GetClaim(ctx, b)
	if err != nil {
		return nil, err
	}
	a := &ipamv1alpha1.IPClaim{}
	if err := json.Unmarshal(b, a); err != nil {
		return nil, err
	}
	return a, nil

}

func (r *bemock) Claim(ctx context.Context, cr client.Object, d any) (*ipamv1alpha1.IPClaim, error) {
	b, err := NormalizeKRMToBytes(cr, d)
	if err != nil {
		return nil, err
	}
	b, err = r.be.Claim(ctx, b)
	if err != nil {
		return nil, err
	}
	a := &ipamv1alpha1.IPClaim{}
	if err := json.Unmarshal(b, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (r *bemock) DeleteClaim(ctx context.Context, cr client.Object, d any) error {
	b, err := NormalizeKRMToBytes(cr, d)
	if err != nil {
		return err
	}
	return r.be.DeleteClaim(ctx, b)
}
