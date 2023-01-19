package ipam

import (
	"net/netip"
	"testing"

	"github.com/hansthienpondt/nipam/pkg/table"
	"github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
)

func Test_validateParentExist(t *testing.T) {

	r := table.NewRoute(
		netip.MustParsePrefix("192.168.0.0/24"),
		map[string]string{},
		map[string]any{
			"foo": "bar",
		},
	)

	pi := iputil.NewPrefixInfo(netip.MustParsePrefix("192.168.0.0/24"))

	// PrefixKindNetwork
	// // pass
	r = r.UpdateLabel(map[string]string{
		ipamv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindAggregate),
	})
	s := validateParentExist(r, v1alpha1.PrefixKindNetwork, pi)
	if s != "" {
		t.Errorf("unexpected error received %q", s)
	}
	// // message
	r = r.UpdateLabel(map[string]string{
		ipamv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindNetwork),
	})
	s = validateParentExist(r, v1alpha1.PrefixKindNetwork, pi)
	if s == "" {
		t.Errorf("no message received, although it was expected to receive one")
	}

	// PrefixKindAggregate
	// // pass
	r = r.UpdateLabel(map[string]string{
		ipamv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindAggregate),
	})
	s = validateParentExist(r, v1alpha1.PrefixKindAggregate, pi)
	if s != "" {
		t.Errorf("unexpected error received %q", s)
	}
	// // message
	r = r.UpdateLabel(map[string]string{
		ipamv1alpha1.NephioPrefixKindKey: "other",
	})
	s = validateParentExist(r, v1alpha1.PrefixKindAggregate, pi)
	if s == "" {
		t.Errorf("no message received, although it was expected to receive one")
	}

	//PrefixKindLoopback
	// // message - nesting loopback prefixes with anything other than an aggregate/loopback prefix is not allowed
	r = r.UpdateLabel(map[string]string{
		ipamv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindNetwork),
	})
	s = validateParentExist(r, v1alpha1.PrefixKindLoopback, pi)
	if s != "" {
		t.Errorf("unexpected error received %q", s)
	}
	// // message
	r = r.UpdateLabel(map[string]string{
		ipamv1alpha1.NephioPrefixKindKey: string(ipamv1alpha1.PrefixKindAggregate),
	})
	s = validateParentExist(r, v1alpha1.PrefixKindLoopback, pi)
	if s == "" {
		t.Errorf("no message received, although it was expected to receive one")
	}
}
