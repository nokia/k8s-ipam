package beclient

import (
	"context"
	"fmt"
	"reflect"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/ipam"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/vlan"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx context.Context, address string) Client {
	return &cl{
		ipam: ipam.New(ctx, clientproxy.Config{Address: address}),
		vlan: vlan.New(ctx, clientproxy.Config{Address: address}),
	}
}

type cl struct {
	ipam clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim]
	vlan clientproxy.Proxy[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim]
}

func (r *cl) CreateIndex(ctx context.Context, cr client.Object) error {
	switch cr.GetObjectKind().GroupVersionKind().Group {
	case ipamv1alpha1.GroupVersion.Group:
		cr, ok := cr.(*ipamv1alpha1.NetworkInstance)
		if !ok {
			return fmt.Errorf("unexpected error casting object expected: %s, got: %v", cr.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(cr))
		}
		return r.ipam.CreateIndex(ctx, cr)
	case vlanv1alpha1.GroupVersion.Group:
		cr, ok := cr.(*vlanv1alpha1.VLANIndex)
		if !ok {
			return fmt.Errorf("unexpected error casting object expected: %s, got: %v", cr.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(cr))
		}
		return r.vlan.CreateIndex(ctx, cr)
	default:
		return fmt.Errorf("unsupported")
	}
}

// Delete deletes the cache instance in the backend
func (r *cl) DeleteIndex(ctx context.Context, cr client.Object) error {
	switch cr.GetObjectKind().GroupVersionKind().Group {
	case ipamv1alpha1.GroupVersion.Group:
		cr, ok := cr.(*ipamv1alpha1.NetworkInstance)
		if !ok {
			return fmt.Errorf("unexpected error casting object expected: %s, got: %v", cr.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(cr))
		}
		return r.ipam.DeleteIndex(ctx, cr)
	case vlanv1alpha1.GroupVersion.Group:
		cr, ok := cr.(*vlanv1alpha1.VLANIndex)
		if !ok {
			return fmt.Errorf("unexpected error casting object expected: %s, got: %v", cr.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(cr))
		}
		return r.vlan.DeleteIndex(ctx, cr)
	default:
		return fmt.Errorf("unsupported")
	}
}

// Get returns the claimed resource
func (r *cl) GetClaim(ctx context.Context, cr client.Object, d any) (client.Object, error) {
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
func (r *cl) Claim(ctx context.Context, cr client.Object, d any) (client.Object, error) {
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
func (r *cl) DeleteClaim(ctx context.Context, cr client.Object, d any) error {
	switch cr.GetObjectKind().GroupVersionKind().Group {
	case ipamv1alpha1.GroupVersion.Group:
		return r.ipam.DeleteClaim(ctx, cr, d)
	case vlanv1alpha1.GroupVersion.Group:
		return r.vlan.DeleteClaim(ctx, cr, d)
	default:
		return fmt.Errorf("unsupported")
	}
}
