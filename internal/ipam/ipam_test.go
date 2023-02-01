package ipam

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type allocation struct {
	kind      string
	name      string
	namespace string
	spec      ipamv1alpha1.IPAllocationSpec
}

func buildNetworkInstance(alloc *allocation) *ipamv1alpha1.NetworkInstance {
	return &ipamv1alpha1.NetworkInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ipamv1alpha1.GroupVersion.String(),
			Kind:       ipamv1alpha1.NetworkInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: alloc.spec.NetworkInstanceRef.Namespace,
			Name:      alloc.spec.NetworkInstanceRef.Name,
		},
	}
}

func buildIPAllocation(alloc *allocation) *ipamv1alpha1.IPAllocation {
	switch alloc.kind {
	case ipamv1alpha1.NetworkInstanceKind:
		return ipamv1alpha1.BuildIPAllocationFromNetworkInstancePrefix(
			&ipamv1alpha1.NetworkInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ipamv1alpha1.GroupVersion.String(),
					Kind:       ipamv1alpha1.NetworkInstanceKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alloc.namespace,
					Name:      alloc.name,
				},
			},
			&ipamv1alpha1.Prefix{
				Prefix: alloc.spec.Prefix,
				Labels: alloc.spec.Labels,
			},
		)
	case ipamv1alpha1.IPPrefixKind:
		return ipamv1alpha1.BuildIPAllocationFromIPPrefix(
			&ipamv1alpha1.IPPrefix{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ipamv1alpha1.GroupVersion.String(),
					Kind:       ipamv1alpha1.IPPrefixKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alloc.namespace,
					Name:      alloc.name,
				},
				Spec: ipamv1alpha1.IPPrefixSpec{
					NetworkInstanceRef: alloc.spec.NetworkInstanceRef,
					PrefixKind:         alloc.spec.PrefixKind,
					Prefix:             alloc.spec.Prefix,
					Labels:             alloc.spec.Labels,
				},
			},
		)
	case ipamv1alpha1.IPAllocationKind:
		return ipamv1alpha1.BuildIPAllocationFromIPAllocation(
			&ipamv1alpha1.IPAllocation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ipamv1alpha1.GroupVersion.String(),
					Kind:       ipamv1alpha1.IPAllocationKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alloc.namespace,
					Name:      alloc.name,
				},
				Spec: alloc.spec,
			},
		)
	}
	return nil
}

var (
	r0 = table.NewRoute(netip.MustParsePrefix("10.0.0.0/8"), map[string]string{
		"nephio.org/nsn-namespace":       "dummy",
		"nephio.org/owner-gvk":           "NetworkInstance.v1alpha1.ipam.nephio.org",
		"nephio.org/owner-nsn-name":      "niName",
		"nephio.org/prefix-kind":         "aggregate",
		"nephio.org/address-family":      "ipv4",
		"nephio.org/nsn-name":            "niName-aggregate-10.0.0.0-8",
		"nephio.org/owner-nsn-namespace": "dummy",
		"nephio.org/gvk":                 "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/subnet":              "10.0.0.0-8",
	}, nil)

	r1 = table.NewRoute(netip.MustParsePrefix("10.0.0.0/24"), map[string]string{
		"nephio.org/owner-nsn-name":      "alloc-net1-prefix1",
		"nephio.org/owner-nsn-namespace": "test",
		"nephio.org/prefix-kind":         "network",
		"nephio.org/network-name":        "net1",
		"nephio.org/address-family":      "ipv4",
		"nephio.org/subnet":              "10.0.0.0-24",
		"nephio.org/gvk":                 "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/nsn-name":            "alloc-net1-prefix1",
		"nephio.org/region":              "us-central1",
		"nephio.org/site":                "edge1",
		"nephio.org/owner-gvk":           "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/nsn-namespace":       "test",
	}, nil)

	r2 = table.NewRoute(netip.MustParsePrefix("10.0.0.0/32"), map[string]string{
		"nephio.org/network-name":        "net1",
		"nephio.org/gvk":                 "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/nsn-name":            "alloc-net1-prefix1",
		"nephio.org/region":              "us-central1",
		"nephio.org/owner-nsn-namespace": "test",
		"nephio.org/owner-gvk":           "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/nsn-namespace":       "test",
		"nephio.org/address-family":      "ipv4",
		"nephio.org/prefix-kind":         "network",
		"nephio.org/subnet":              "10.0.0.0-24",
		"nephio.org/site":                "edge1",
		"nephio.org/owner-nsn-name":      "alloc-net1-prefix1",
	}, nil)

	r3 = table.NewRoute(netip.MustParsePrefix("10.0.0.1/32"), map[string]string{
		"nephio.org/owner-nsn-name":      "alloc-net1-prefix1",
		"nephio.org/prefix-kind":         "network",
		"nephio.org/subnet":              "10.0.0.0-24",
		"nephio.org/region":              "us-central1",
		"nephio.org/owner-gvk":           "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/network-name":        "net1",
		"nephio.org/owner-nsn-namespace": "test",
		"nephio.org/gvk":                 "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/nsn-name":            "alloc-net1-prefix1",
		"nephio.org/site":                "edge1",
		"nephio.org/address-family":      "ipv4",
		"nephio.org/gateway":             "true",
		"nephio.org/nsn-namespace":       "test",
	}, nil)

	r4 = table.NewRoute(netip.MustParsePrefix("10.0.0.2/32"), map[string]string{
		"nephio.org/owner-nsn-name":      "alloc-net1-staticprefix",
		"nephio.org/gateway":             "true",
		"nephio.org/address-family":      "ipv4",
		"nephio.org/subnet":              "10.0.0.0-24",
		"nephio.org/owner-nsn-namespace": "test",
		"nephio.org/nsn-name":            "alloc-net1-staticprefix",
		"nephio.org/nsn-namespace":       "test",
		"nephio.org/region":              "us-central1",
		"nephio.org/site":                "edge1",
		"nephio.org/network-name":        "net1",
		"nephio.org/owner-gvk":           "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/gvk":                 "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/prefix-kind":         "network",
	}, nil)

	r5 = table.NewRoute(netip.MustParsePrefix("10.0.0.3/32"), map[string]string{
		"nephio.org/owner-nsn-namespace": "test",
		"nephio.org/nsn-namespace":       "test",
		"nephio.org/address-family":      "ipv4",
		"nephio.org/owner-gvk":           "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/owner-nsn-name":      "alloc-net1-alloc1",
		"nephio.org/gvk":                 "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/nsn-name":            "alloc-net1-alloc1",
		"nephio.org/prefix-kind":         "network",
		"nephio.org/subnet":              "10.0.0.0-24",
	}, nil)

	r6 = table.NewRoute(netip.MustParsePrefix("10.0.0.255/32"), map[string]string{
		"nephio.org/owner-nsn-name":      "alloc-net1-prefix1",
		"nephio.org/subnet":              "10.0.0.0-24",
		"nephio.org/gvk":                 "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/region":              "us-central1",
		"nephio.org/site":                "edge1",
		"nephio.org/owner-gvk":           "IPAllocation.v1alpha1.ipam.nephio.org",
		"nephio.org/owner-nsn-namespace": "test",
		"nephio.org/prefix-kind":         "network",
		"nephio.org/nsn-name":            "alloc-net1-prefix1",
		"nephio.org/address-family":      "ipv4",
		"nephio.org/network-name":        "net1",
		"nephio.org/nsn-namespace":       "test",
	}, nil)
)

func TestNetworkInstance(t *testing.T) {
	niRef := &ipamv1alpha1.NetworkInstanceReference{
		Namespace: "dummy",
		Name:      "niName",
	}
	allocNamespace := "test"
	niCreate := &allocation{spec: ipamv1alpha1.IPAllocationSpec{NetworkInstanceRef: niRef}}

	type ipamTests struct {
		allocation *allocation
		errString  string
		gateway    string
	}

	tests := []ipamTests{
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.NetworkInstanceKind,
				namespace: niRef.Namespace,
				name:      niRef.Name,
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					Prefix:             "10.0.0.0/8",
				},
			},
			errString: "",
		},
		// create network prefix w/o parent
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Prefix:             "10.0.0.2/24",
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: errValidateNetworkPrefixWoNetworkParent,
		},
		// create parent prefix
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Prefix:             "10.0.0.1/24",
					CreatePrefix:       true,
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: "",
		},
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-staticprefix",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Prefix:             "10.0.0.2/24",
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: errValidateNetworkPrefixWoNetworkParent,
		},
		// test duplication
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix2",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Prefix:             "10.0.0.10/24",
					CreatePrefix:       true,
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: errValidateDuplicatePrefix,
		},
		// test allocation
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-alloc1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nephio.org/region":       "us-central1",
							"nephio.org/site":         "edge1",
							"nephio.org/network-name": "net1",
						},
					},
				},
			},
			errString: "",
			gateway:   "10.0.0.1",
		},
	}

	// create new rib
	ipam := New(nil)
	// create new networkinstance
	niCr := buildNetworkInstance(niCreate)
	if err := ipam.Create(context.Background(), niCr); err != nil {
		t.Errorf("%v occured, cannot create network instance: %s/%s", err, niCr.GetNamespace(), niCr.GetName())
	}

	for _, test := range tests {
		allocReq := buildIPAllocation(test.allocation)
		allocResp, err := ipam.AllocateIPPrefix(context.Background(), allocReq)
		if err != nil {
			if test.errString == "" {
				t.Errorf("%v, cannot create ip prefix: %v", err, allocResp)
				return
			} else {
				// expected error
				if !strings.Contains(err.Error(), test.errString) {
					t.Errorf("expecting error: %s, got %v resp: %v", test.errString, err, allocResp)
					return
				}
			}
		}
		if test.errString == "" {
			if test.allocation.spec.Prefix != "" {
				if allocResp.Status.AllocatedPrefix != test.allocation.spec.Prefix {
					t.Errorf("expected prefix %s, got %s", test.allocation.spec.Prefix, allocResp.Status.AllocatedPrefix)
				}
			} else {
				if allocResp.Status.AllocatedPrefix == "" || allocResp.Status.Gateway != test.gateway {
					t.Errorf("expected allocation with gateway %s, got allocatedPrefix %s, gateway %s", test.gateway, allocResp.Status.AllocatedPrefix, allocReq.Status.Gateway)
				}
			}
		}
	}

	routes := ipam.GetPrefixes(niCr)
	comparationRoutes := []table.Route{r0, r1, r2, r3, r4, r5, r6}

	for i, route := range routes {
		if !route.Equal(comparationRoutes[i]) {
			t.Errorf("route %d is not equal [%s] vs [%s]", i, route.String(), comparationRoutes[i].String())
		}
	}

}
