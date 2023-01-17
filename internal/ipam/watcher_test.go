package ipam

import (
	"context"
	"net/netip"
	"testing"

	"github.com/hansthienpondt/nipam/pkg/table"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
)

func Test_watcher_handleUpdate(t *testing.T) {
	w := newWatcher()
	gvkKey := "foo"
	ownerGvk := "bar"
	isCalled := false
	route1 := table.NewRoute(
		netip.MustParsePrefix("192.168.0.1/24"),
		map[string]string{"key1": "bar"},
		map[string]any{"bla": "blubb"},
	)
	route2 := table.NewRoute(
		netip.MustParsePrefix("1.2.3.1/24"),
		map[string]string{"key1": "prefix"},
		map[string]any{"bla": "other"},
	)
	routes := table.Routes{
		route1,
		route2,
	}
	allocStatus := allocpb.StatusCode_Valid
	fn := func(fnRoutes table.Routes, sc allocpb.StatusCode) {
		isCalled = true
		if fnRoutes.Len() != len(routes) {
			t.Errorf("expected %d routes, got %d", len(routes), fnRoutes.Len())
		}
		if sc != allocStatus {
			t.Errorf("expected %q but got %q statuscode", allocStatus, sc)
		}
	}

	// setup the watch
	w.addWatch(gvkKey, ownerGvk, fn)
	ctx := context.TODO()

	// trigger update handling
	w.handleUpdate(ctx, routes, allocStatus)

	// check that the callback was triggered
	if !isCalled {
		t.Errorf("the callbackfn was expected to be executed but wasn't")
	}

	// reset isCalled
	isCalled = false

	// finally delete the watch
	w.deleteWatch(gvkKey, ownerGvk)

	// handleUpdate again should not set isCalled to true
	w.handleUpdate(ctx, routes, allocStatus)

	if isCalled {
		t.Errorf("although the watcher was deleted, it was triggered")
	}
}
