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
	"fmt"
	"reflect"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func NewMock() clientproxy.Proxy[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim] {
	return &mock{}
}

type mock struct{}

func (r *mock) AddEventChs(map[schema.GroupVersionKind]chan event.GenericEvent)   {}
func (r *mock) CreateIndex(ctx context.Context, cr *vlanv1alpha1.VLANIndex) error { return nil }
func (r *mock) DeleteIndex(ctx context.Context, cr *vlanv1alpha1.VLANIndex) error { return nil }
func (r *mock) GetClaim(ctx context.Context, cr client.Object, d any) (*vlanv1alpha1.VLANClaim, error) {
	return r.getClaim(cr)
}
func (r *mock) Claim(ctx context.Context, cr client.Object, d any) (*vlanv1alpha1.VLANClaim, error) {
	return r.getClaim(cr)
}
func (r *mock) DeleteClaim(ctx context.Context, cr client.Object, d any) error { return nil }

func (r *mock) getClaim(cr client.Object) (*vlanv1alpha1.VLANClaim, error) {
	claim, ok := cr.(*vlanv1alpha1.VLANClaim)
	if !ok {
		return nil, fmt.Errorf("expecting VLANClaim, got: %v", reflect.TypeOf(cr))
	}
	claim.Status.VLANID = util.PointerUint16(10)
	return claim, nil
}
