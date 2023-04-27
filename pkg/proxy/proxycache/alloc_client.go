package proxycache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/nokia/k8s-ipam/pkg/alloc/alloc"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
)

func (r *proxycache) getClient() (allocpb.AllocationClient, error) {
	r.m.RLock()
	defer r.m.RUnlock()
	if r.allocClient == nil {
		return nil, fmt.Errorf("ipam server unreachable")
	}
	return r.allocClient.Get(), nil
}

func (r *proxycache) deleteClient(ctx context.Context) error {
	r.m.Lock()
	defer r.m.Unlock()
	if r.allocClient != nil {
		// cancel the watch
		r.stopWatches()
		if err := r.allocClient.Delete(); err != nil {
			r.l.Error(err, "cannot delete client")
			return err
		}
	}
	r.allocClient = nil
	return nil
}

func (r *proxycache) createClient(ctx context.Context) error {
	r.m.Lock()
	defer r.m.Unlock()
	ac, err := alloc.New(&alloc.Config{
		Address:  fmt.Sprintf("%s:%s", r.svcInfo.Address, strconv.Itoa(r.svcInfo.Port)),
		Insecure: true,
	})
	if err != nil {
		r.l.Error(err, "cannot create client")
		r.allocClient = nil
		return err
	}

	r.startWatches(ctx)

	r.allocClient = ac
	return nil
}
