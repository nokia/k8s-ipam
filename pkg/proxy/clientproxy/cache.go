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
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Cache interface {
	Get(ObjectKindKey) *resourcepb.ClaimResponse
	Add(ObjectKindKey, *resourcepb.ClaimResponse)
	Delete(ObjectKindKey)
	ValidateExpiryTime(context.Context) map[ObjectKindKey]*resourcepb.ClaimResponse
}

func NewCache() Cache {
	return &cache{
		c: map[ObjectKindKey]*resourcepb.ClaimResponse{},
	}
}

// ObjectKindKey is the key of the Claim, not the ownerKey
type ObjectKindKey struct {
	gvk schema.GroupVersionKind
	nsn types.NamespacedName
}

type cache struct {
	m sync.RWMutex
	c map[ObjectKindKey]*resourcepb.ClaimResponse
	l logr.Logger
}

func (r *cache) Get(key ObjectKindKey) *resourcepb.ClaimResponse {
	r.m.RLock()
	defer r.m.RUnlock()
	if claimRsp, ok := r.c[key]; ok {
		return claimRsp
	}
	return nil
}

func (r *cache) Add(key ObjectKindKey, claimRsp *resourcepb.ClaimResponse) {
	r.m.Lock()
	defer r.m.Unlock()
	r.c[key] = claimRsp
}

func (r *cache) Delete(key ObjectKindKey) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.c, key)
}

func getKey(claim *resourcepb.ClaimRequest) ObjectKindKey {
	return ObjectKindKey{
		gvk: schema.GroupVersionKind{
			Group:   claim.Header.Gvk.Group,
			Version: claim.Header.Gvk.Version,
			Kind:    claim.Header.Gvk.Kind,
		},
		nsn: types.NamespacedName{
			Namespace: claim.Header.Nsn.Namespace,
			Name:      claim.Header.Nsn.Name,
		},
	}
}

func isCacheDataValid(cacheResp *resourcepb.ClaimResponse, claim *resourcepb.ClaimRequest) bool {
	if cacheResp.Spec != claim.Spec {
		return false
	}
	if cacheResp.StatusCode != resourcepb.StatusCode_Valid {
		return false
	}
	return true
}

func (r *cache) ValidateExpiryTime(ctx context.Context) map[ObjectKindKey]*resourcepb.ClaimResponse {
	r.l = log.FromContext(ctx)
	r.m.RLock()
	defer r.m.RUnlock()
	claimsToRefresh := map[ObjectKindKey]*resourcepb.ClaimResponse{}
	for key, resourcepbResp := range r.c {
		if resourcepbResp.ExpiryTime != "never" && resourcepbResp.StatusCode == resourcepb.StatusCode_Valid {
			t, err := time.Parse(time.RFC3339, resourcepbResp.ExpiryTime)
			if err != nil {
				r.l.Error(err, "cannot unmarshal expirytime", "key", key)
				continue
			}
			minRefreshTime := time.Now().Add(time.Minute * 59)
			r.l.Info("expiry", "now", time.Now(), "expiryTime", t, "minRefreshTime", minRefreshTime, "delta", t.Sub(minRefreshTime), "key", key.gvk, "nsn", key.nsn)
			if t.Sub(minRefreshTime) < 0 {
				// add key to keys to be refreshed
				claimsToRefresh[key] = resourcepbResp
			}
		}
	}
	return claimsToRefresh
}
