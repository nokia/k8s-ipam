package ipam

import (
	"context"
	"net/netip"
	"testing"

	"github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
)

func TestMutateAllocNetworkWithPrefixFirst(t *testing.T) {

	// prepare input for instantiation of an IPAllocation
	namespace := "mynamespace"
	labels := map[string]string{"foo": "bar"}
	crname := "mycrname"
	ipallocSpec := v1alpha1.IPAllocationSpec{}
	ipallocStatus := v1alpha1.IPAllocationStatus{}
	a := v1alpha1.BuildIPAllocation(namespace, labels, crname, ipallocSpec, ipallocStatus)

	pi0 := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.5.0/24"))

	// create a new mutator
	m := NewMutator(
		&MutatorConfig{
			alloc: a,
			pi:    pi0,
		},
	)
	// create a contexr
	ctx := context.TODO()

	// prefixkind != Network and Prefix == ""
	a.Spec.PrefixKind = v1alpha1.PrefixKindLoopback
	a.Spec.Prefix = ""
	a.Spec.AddressFamily = iputil.AddressFamilyIpv4
	a.Spec.PrefixLength = 30

	// call the mutator MutateAllocNetworkWithPrefix function
	ipAllocs := m.MutateAllocNetworkWithPrefix(ctx)
	if len(ipAllocs) != 3 {
		t.Errorf("expected 3 allocation, got %d", len(ipAllocs))
	}
	for _, x := range ipAllocs {
		if x.GetAddressFamily() != a.Spec.AddressFamily {
			t.Errorf("expecting same address family on alloc (%q) as on spec (%q)", x.GetAddressFamily(), a.Spec.AddressFamily)
		}
		expectedLabelCount := 1
		if len(x.GetLabels()) != expectedLabelCount {
			t.Errorf("expected %d labels got %d", expectedLabelCount, len(x.GetLabels()))
		}
	}
}

func TestMutateAllocNetworkWithPrefixNotFirstNorLast(t *testing.T) {

	// prepare input for instantiation of an IPAllocation
	namespace := "mynamespace"
	labels := map[string]string{"foo": "bar"}
	crname := "mycrname"
	ipallocSpec := v1alpha1.IPAllocationSpec{}
	ipallocStatus := v1alpha1.IPAllocationStatus{}
	a := v1alpha1.BuildIPAllocation(namespace, labels, crname, ipallocSpec, ipallocStatus)

	pi0 := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.5.22/24"))

	// create a new mutator
	m := NewMutator(
		&MutatorConfig{
			alloc: a,
			pi:    pi0,
		},
	)
	// create a contexr
	ctx := context.TODO()

	// prefixkind != Network and Prefix == ""
	a.Spec.PrefixKind = v1alpha1.PrefixKindAggregate
	a.Spec.Prefix = ""
	a.Spec.AddressFamily = iputil.AddressFamilyIpv4
	a.Spec.PrefixLength = 30

	// call the mutator MutateAllocNetworkWithPrefix function
	ipAllocs := m.MutateAllocNetworkWithPrefix(ctx)
	expected := 4
	if len(ipAllocs) != expected {
		t.Errorf("expected %d allocation, got %d", expected, len(ipAllocs))
	}
	for _, x := range ipAllocs {
		if x.GetAddressFamily() != a.Spec.AddressFamily {
			t.Errorf("expecting same address family on alloc (%q) as on spec (%q)", x.GetAddressFamily(), a.Spec.AddressFamily)
		}
	}
}

func TestMutateAllocNetworkWithPrefixLast(t *testing.T) {

	// prepare input for instantiation of an IPAllocation
	namespace := "mynamespace"
	labels := map[string]string{"foo": "bar"}
	crname := "mycrname"
	ipallocSpec := v1alpha1.IPAllocationSpec{}
	ipallocStatus := v1alpha1.IPAllocationStatus{}
	a := v1alpha1.BuildIPAllocation(namespace, labels, crname, ipallocSpec, ipallocStatus)

	pi0 := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.5.255/24"))

	// create a new mutator
	m := NewMutator(
		&MutatorConfig{
			alloc: a,
			pi:    pi0,
		},
	)
	// create a contexr
	ctx := context.TODO()

	// prefixkind != Network and Prefix == ""
	a.Spec.PrefixKind = v1alpha1.PrefixKindAggregate
	a.Spec.Prefix = ""
	a.Spec.AddressFamily = iputil.AddressFamilyIpv4
	a.Spec.PrefixLength = 30

	// call the mutator MutateAllocNetworkWithPrefix function
	ipAllocs := m.MutateAllocNetworkWithPrefix(ctx)
	if len(ipAllocs) != 3 {
		t.Errorf("expected 3 allocation, got %d", len(ipAllocs))
	}
	for _, x := range ipAllocs {
		if x.GetAddressFamily() != a.Spec.AddressFamily {
			t.Errorf("expecting same address family on alloc (%q) as on spec (%q)", x.GetAddressFamily(), a.Spec.AddressFamily)
		}
	}
}

func TestMutatenetworkNet(t *testing.T) {

	// prepare input for instantiation of an IPAllocation
	namespace := "mynamespace"
	labels := map[string]string{
		"foo":                         "bar",
		ipamv1alpha1.NephioGatewayKey: "true",
	}
	crname := "mycrname"
	ipallocSpec := v1alpha1.IPAllocationSpec{}
	ipallocStatus := v1alpha1.IPAllocationStatus{}
	a := v1alpha1.BuildIPAllocation(namespace, labels, crname, ipallocSpec, ipallocStatus)

	pi0 := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.5.1/24"))

	// create a new mutator
	m := &mutator{
		alloc: a,
		pi:    pi0,
	}

	a.Spec.Labels = labels
	newalloc := m.mutateNetworkNet(a)
	if _, exists := newalloc.Spec.Labels[ipamv1alpha1.NephioGatewayKey]; exists {
		t.Errorf("Label %q should not exist on NetworkNet", ipamv1alpha1.NephioGatewayKey)
	}
}

func TestMutateNetworkFirstAddressInNet(t *testing.T) {

	// prepare input for instantiation of an IPAllocation
	namespace := "mynamespace"
	labels := map[string]string{
		"foo":                         "bar",
		ipamv1alpha1.NephioGatewayKey: "true",
	}
	crname := "mycrname"
	ipallocSpec := v1alpha1.IPAllocationSpec{}
	ipallocStatus := v1alpha1.IPAllocationStatus{}
	a := v1alpha1.BuildIPAllocation(namespace, labels, crname, ipallocSpec, ipallocStatus)

	pi0 := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.5.1/24"))

	// create a new mutator
	m := &mutator{
		alloc: a,
		pi:    pi0,
	}

	a.Spec.Labels = labels
	newalloc := m.mutateNetworkFirstAddressInNet(a)
	if _, exists := newalloc.Spec.Labels[ipamv1alpha1.NephioGatewayKey]; exists {
		t.Errorf("Label %q should not exist on NetworkNet", ipamv1alpha1.NephioGatewayKey)
	}

}

func TestMutateAllocWithoutPrefix(t *testing.T) {
	// prepare input for instantiation of an IPAllocation
	namespace := "mynamespace"
	labels := map[string]string{
		"foo":                         "bar",
		ipamv1alpha1.NephioGatewayKey: "true",
	}
	crname := "mycrname"
	ipallocSpec := v1alpha1.IPAllocationSpec{}
	ipallocStatus := v1alpha1.IPAllocationStatus{}
	a := v1alpha1.BuildIPAllocation(namespace, labels, crname, ipallocSpec, ipallocStatus)

	pi0 := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.5.1/24"))

	// create a new mutator
	m := &mutator{
		alloc: a,
		pi:    pi0,
	}

	// create a contexr
	ctx := context.TODO()

	// prefixkind != Network and Prefix == ""
	a.Spec.PrefixKind = v1alpha1.PrefixKindAggregate
	a.Spec.Prefix = ""
	a.Spec.AddressFamily = iputil.AddressFamilyIpv4
	a.Spec.PrefixLength = 30
	ipAllocs := m.MutateAllocWithoutPrefix(ctx)
	expected := 1
	if len(ipAllocs) != expected {
		t.Errorf("expected %d allocsations, got %d", expected, len(ipAllocs))
	}
}

func TestMutateAllocWithPrefix(t *testing.T) {
	// prepare input for instantiation of an IPAllocation
	namespace := "mynamespace"
	labels := map[string]string{
		"foo":                         "bar",
		ipamv1alpha1.NephioGatewayKey: "true",
	}
	crname := "mycrname"
	ipallocSpec := v1alpha1.IPAllocationSpec{}
	ipallocStatus := v1alpha1.IPAllocationStatus{}
	a := v1alpha1.BuildIPAllocation(namespace, labels, crname, ipallocSpec, ipallocStatus)

	pi0 := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.5.1/24"))

	// create a new mutator
	m := &mutator{
		alloc: a,
		pi:    pi0,
	}

	// create a contexr
	ctx := context.TODO()

	// prefixkind != Network and Prefix == ""
	a.Spec.PrefixKind = v1alpha1.PrefixKindNetwork
	a.Spec.Prefix = ""
	a.Spec.AddressFamily = iputil.AddressFamilyIpv4
	a.Spec.PrefixLength = 30
	ipAllocs := m.MutateAllocWithPrefix(ctx)
	expected := 1
	if len(ipAllocs) != expected {
		t.Errorf("expected %d allocsations, got %d", expected, len(ipAllocs))
	}
	// check the labels for PrefixKindNetwork
	_, existsNephioPrefixLengthKey := ipAllocs[0].Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey]
	_, existsNephioParentPrefixLengthKey := ipAllocs[0].Spec.Labels[ipamv1alpha1.NephioParentPrefixLengthKey]
	_, existsNephioSubnetKey := ipAllocs[0].Spec.Labels[ipamv1alpha1.NephioSubnetKey]

	if !(existsNephioParentPrefixLengthKey && existsNephioPrefixLengthKey && existsNephioSubnetKey) {
		t.Errorf("missing labels for PrefixKindNetwork")
	}
}
