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

package vlan

import (
	"context"
	"encoding/json"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/vlanbackend"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func NewBackendMock() clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation] {
	be, _ := vlanbackend.New(nil)
	return &bemock{
		be: be,
	}
}

type bemock struct {
	be backend.Backend
}

func (r *bemock) AddEventChs(map[schema.GroupVersionKind]chan event.GenericEvent) {}

func (r *bemock) CreateIndex(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	return r.be.CreateIndex(ctx, b)
}

func (r *bemock) DeleteIndex(ctx context.Context, cr *vlanv1alpha1.VLANDatabase) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	return r.be.DeleteIndex(ctx, b)
}

func (r *bemock) GetAllocation(ctx context.Context, cr client.Object, d any) (*vlanv1alpha1.VLANAllocation, error) {
	b, err := json.Marshal(cr)
	if err != nil {
		return nil, err
	}
	b, err = r.be.GetAllocation(ctx, b)
	if err != nil {
		return nil, err
	}
	a := &vlanv1alpha1.VLANAllocation{}
	if err := json.Unmarshal(b, a); err != nil {
		return nil, err
	}
	return a, nil

}

func (r *bemock) Allocate(ctx context.Context, cr client.Object, d any) (*vlanv1alpha1.VLANAllocation, error) {
	b, err := json.Marshal(cr)
	if err != nil {
		return nil, err
	}
	b, err = r.be.Allocate(ctx, b)
	if err != nil {
		return nil, err
	}
	a := &vlanv1alpha1.VLANAllocation{}
	if err := json.Unmarshal(b, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (r *bemock) DeAllocate(ctx context.Context, cr client.Object, d any) error {
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	return r.be.DeAllocate(ctx, b)
}
