package beclient

import (
	"context"
	"fmt"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/ipam"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/vlan"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewMock() Client {
	return &mock{
		ipam: ipam.NewMock(),
		vlan: vlan.NewMock(),
	}
}

type mock struct {
	ipam clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim]
	vlan clientproxy.Proxy[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim]
}

func (r *mock) CreateIndex(ctx context.Context, cr client.Object) error {
	return fmt.Errorf("unsupported")
}

// Delete deletes the cache instance in the backend
func (r *mock) DeleteIndex(ctx context.Context, cr client.Object) error {
	return fmt.Errorf("unsupported")
}

// Get returns the claimed resource
func (r *mock) GetClaim(ctx context.Context, cr client.Object, d any) (client.Object, error) {
	switch cr.GetObjectKind().GroupVersionKind().Group {
	case ipamv1alpha1.GroupVersion.Group:
		return r.ipam.GetClaim(ctx, cr, d)
	case vlanv1alpha1.GroupVersion.Group:
		return r.vlan.GetClaim(ctx, cr, d)
	default:
		return nil, fmt.Errorf("unsupported")
	}
}

// Claim claims a resource
func (r *mock) Claim(ctx context.Context, cr client.Object, d any) (client.Object, error) {
	switch cr.GetObjectKind().GroupVersionKind().Group {
	case ipamv1alpha1.GroupVersion.Group:
		return r.ipam.Claim(ctx, cr, d)
	case vlanv1alpha1.GroupVersion.Group:
		return r.vlan.Claim(ctx, cr, d)
	default:
		return nil, fmt.Errorf("unsupported")
	}
}

// DeleteClaim deletes the claim
func (r *mock) DeleteClaim(ctx context.Context, cr client.Object, d any) error {
	return fmt.Errorf("unsupported")
}
