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

package clientproxy

import (
	"context"
	"fmt"

	"github.com/nokia/k8s-ipam/pkg/proto/resource"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
)

func (r *clientproxy[T1, T2]) getClient() (resourcepb.ResourceClient, error) {
	r.m.RLock()
	defer r.m.RUnlock()
	if r.resourceClient == nil {
		return nil, fmt.Errorf("backend server unreachable")
	}
	return r.resourceClient.Get(), nil
}

func (r *clientproxy[T1, T2]) deleteClient(ctx context.Context) error {
	r.m.Lock()
	defer r.m.Unlock()
	if r.resourceClient != nil {
		// cancel the watch
		r.stopWatches()
		if err := r.resourceClient.Delete(); err != nil {
			r.l.Error(err, "cannot delete client")
			return err
		}
	}
	r.resourceClient = nil
	return nil
}

func (r *clientproxy[T1, T2]) createClient(ctx context.Context) error {
	r.m.Lock()
	defer r.m.Unlock()
	r.l.Info("create client", "address", r.address)
	ac, err := resource.New(&resource.Config{
		Address:  r.address,
		Insecure: true,
	})
	if err != nil {
		r.l.Error(err, "cannot create client")
		r.resourceClient = nil
		return err
	}

	r.startWatches(ctx)

	r.resourceClient = ac
	return nil
}
