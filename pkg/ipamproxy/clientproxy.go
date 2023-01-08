package ipamproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/proxycache"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IpamClientProxy interface {
	// Create creates the network instance in the ipam
	Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
	// Delete deletes the network instance in the ipam
	Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
	// AllocateIPPrefix allocates an ip prefix
	AllocateIPPrefix(ctx context.Context, cr client.Object, d any) (*AllocatedPrefix, error)
	// DeAllocateIPPrefix
	DeAllocateIPPrefix(ctx context.Context, cr client.Object, d any) error
}

type AllocatedPrefix struct {
	Prefix  string
	Gateway string
}

type ClientConfig struct {
	ProxyCache proxycache.ProxyCache
}

func NewClientProxy(c *ClientConfig) IpamClientProxy {
	l := ctrl.Log.WithName("ipam-client-proxy")
	return &clientproxy{
		pc: c.ProxyCache,
		l:  l,
	}
}

type clientproxy struct {
	pc proxycache.ProxyCache
	//logger
	l logr.Logger
}

func (r *clientproxy) Create(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)
	gvk := meta.GetGVKFromObject(cr)
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	req := buildAllocPb(cr, string(b), "never", gvk, ownerGvk)
	_, err = r.pc.Allocate(ctx, req)
	return err
}

func (r *clientproxy) Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)
	gvk := meta.GetGVKFromObject(cr)
	b, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	req := buildAllocPb(cr, string(b), "never", gvk, ownerGvk)
	return r.pc.DeAllocate(ctx, req)
}

func (r *clientproxy) AllocateIPPrefix(ctx context.Context, o client.Object, d any) (*AllocatedPrefix, error) {
	r.l.Info("allocate prefix", "cr", o)
	// normalizes the input to the proxycache generalized allocation
	req, err := NormalizeKRMToProxyCacheAllocation(o, d)
	if err != nil {
		return nil, err
	}

	resp, err := r.pc.Allocate(ctx, req)
	if err != nil {
		return nil, err
	}
	ipAlloc := ipamv1alpha1.IPAllocation{}
	if err := json.Unmarshal([]byte(resp.Status), &ipAlloc); err != nil {
		return nil, err
	}
	r.l.Info("allocate prefix done", "result", ipAlloc.Status)
	return &AllocatedPrefix{
		Prefix:  ipAlloc.Status.AllocatedPrefix,
		Gateway: ipAlloc.Status.Gateway,
	}, nil

}

func (r *clientproxy) DeAllocateIPPrefix(ctx context.Context, o client.Object, d any) error {
	// normalizes the input to the proxycache generalized allocation
	req, err := NormalizeKRMToProxyCacheAllocation(o, d)
	if err != nil {
		return err
	}
	return r.pc.DeAllocate(ctx, req)
}

// NormalizeKRMToProxyCacheAllocation normalizes the input to a genralized allocation request
func NormalizeKRMToProxyCacheAllocation(o client.Object, d any) (*allocpb.Request, error) {
	switch o.GetObjectKind().GroupVersionKind().Kind {
	case ipamv1alpha1.IPPrefixKind:
		cr, ok := o.(*ipamv1alpha1.IPPrefix)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to IPPrefix failed")
		}
		return BuildAllocationFromIPPrefix(cr)
	case ipamv1alpha1.IPAllocationKind:
		cr, ok := o.(*ipamv1alpha1.IPAllocation)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to IPAllocation failed")
		}
		return BuildAllocationFromIPAllocation(cr, time.Now().UTC().Add(time.Minute*60).String())
	case ipamv1alpha1.NetworkInstanceKind:
		cr, ok := o.(*ipamv1alpha1.NetworkInstance)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to NetworkInstance failed")
		}
		ipPrefix, ok := d.(*ipamv1alpha1.Prefix)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to Ip Prefix failed")
		}
		return BuildAllocationFromNetworkInstancePrefix(cr, ipPrefix)
	default:
		return nil, fmt.Errorf("cannot allocate prefix for unknown kind, got %s", o.GetObjectKind().GroupVersionKind().Kind)
	}
}

func BuildAllocationFromIPPrefix(cr *ipamv1alpha1.IPPrefix) (*allocpb.Request, error) {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	ipalloc := ipamv1alpha1.BuildIPAllocationFromIPPrefix(cr)
	b, err := json.Marshal(ipalloc)
	if err != nil {
		return nil, err
	}

	return buildAllocPb(cr, string(b), "never", getIPAllocGVK(), ownerGvk), nil
}

func BuildAllocationFromNetworkInstancePrefix(cr *ipamv1alpha1.NetworkInstance, prefix *ipamv1alpha1.Prefix) (*allocpb.Request, error) {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)
	ipalloc := ipamv1alpha1.BuildIPAllocationFromNetworkInstancePrefix(cr, prefix)
	b, err := json.Marshal(ipalloc)
	if err != nil {
		return nil, err
	}

	return buildAllocPb(cr, string(b), "never", getIPAllocGVK(), ownerGvk), nil
}

func BuildAllocationFromIPAllocation(cr *ipamv1alpha1.IPAllocation, expiryTime string) (*allocpb.Request, error) {
	// if the ownerGvk is in the labels we use this as ownerGVK
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)
	ownerValue, ok := cr.GetLabels()[ipamv1alpha1.NephioOwnerKey]
	if ok {
		ownerGvk = meta.StringToGVK(ownerValue)
	}

	spec := cr.Spec
	spec.Labels = ipamv1alpha1.GetSpecLabelWithOwnerGVK(ownerGvk, cr.Spec.Labels)

	ipalloc := ipamv1alpha1.BuildIPAllocation(cr, cr.GetName(), spec)
	b, err := json.Marshal(ipalloc)
	if err != nil {
		return nil, err
	}

	return buildAllocPb(cr, string(b), expiryTime, getIPAllocGVK(), ownerGvk), nil
}

func getIPAllocGVK() *schema.GroupVersionKind {
	return &schema.GroupVersionKind{
		Group:   ipamv1alpha1.GroupVersion.Group,
		Version: ipamv1alpha1.GroupVersion.Version,
		Kind:    ipamv1alpha1.IPAllocationKind,
	}
}

func buildAllocPb(o client.Object, specBody string, expiryTime string, gvk, ownerGvk *schema.GroupVersionKind) *allocpb.Request {
	return &allocpb.Request{
		Header: &allocpb.Header{
			Gvk: &allocpb.GVK{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind,
			},
			Nsn: &allocpb.NSN{
				Namespace: o.GetNamespace(),
				Name:      o.GetName(),
			},
			OwnerGvk: &allocpb.GVK{
				Group:   ownerGvk.Group,
				Version: ownerGvk.Version,
				Kind:    ownerGvk.Kind,
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

func GetNameFromNetworkInstancePrefix(name, prefix string) string {
	return fmt.Sprintf("%s-%s-%s", name, "aggregate", strings.ReplaceAll(prefix, "/", "-"))
}
