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
	"fmt"
	"reflect"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func NewMock() clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim] {
	return &mock{}
}

type mock struct{}

func (r *mock) AddEventChs(map[schema.GroupVersionKind]chan event.GenericEvent)         {}
func (r *mock) CreateIndex(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error { return nil }
func (r *mock) DeleteIndex(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error { return nil }
func (r *mock) GetClaim(ctx context.Context, cr client.Object, d any) (*ipamv1alpha1.IPClaim, error) {
	return r.getClaim(cr)
}
func (r *mock) Claim(ctx context.Context, cr client.Object, d any) (*ipamv1alpha1.IPClaim, error) {
	return r.getClaim(cr)
}
func (r *mock) DeleteClaim(ctx context.Context, cr client.Object, d any) error { return nil }

func (r *mock) getClaim(cr client.Object) (*ipamv1alpha1.IPClaim, error) {
	claim, ok := cr.(*ipamv1alpha1.IPClaim)
	if !ok {
		return nil, fmt.Errorf("expecting IPClaim, got: %v", reflect.TypeOf(cr))
	}
	switch claim.Spec.Kind {
	case ipamv1alpha1.PrefixKindNetwork:
		if claim.Spec.AddressFamily != nil && *claim.Spec.AddressFamily == iputil.AddressFamilyIpv6 {
			claim.Status = ipamv1alpha1.IPClaimStatus{Prefix: ptr.To("1000::2/64"), Gateway: ptr.To("1000::1")}
		} else {
			claim.Status = ipamv1alpha1.IPClaimStatus{Prefix: ptr.To("10.0.0.10/24"), Gateway: ptr.To("10.0.0.1")}
		}

	case ipamv1alpha1.PrefixKindLoopback:
		if claim.Spec.AddressFamily != nil && *claim.Spec.AddressFamily == iputil.AddressFamilyIpv6 {
			claim.Status = ipamv1alpha1.IPClaimStatus{Prefix: ptr.To("2000::2/128")}
		} else {
			claim.Status = ipamv1alpha1.IPClaimStatus{Prefix: ptr.To("172.0.0.10/32")}
		}
	case ipamv1alpha1.PrefixKindPool:
		if claim.Spec.AddressFamily != nil && *claim.Spec.AddressFamily == iputil.AddressFamilyIpv6 {
			claim.Status = ipamv1alpha1.IPClaimStatus{Prefix: ptr.To("3000::/32")}
		} else {
			claim.Status = ipamv1alpha1.IPClaimStatus{Prefix: ptr.To("172.0.0.0/8")}
		}

	default:
		return nil, fmt.Errorf("unexpected prefix kind: got: %s", claim.Spec.Kind)
	}
	return claim, nil
}
