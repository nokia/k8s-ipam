package proxycache

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Cache interface {
	Get(ObjectKindKey) *allocpb.AllocResponse
	Add(ObjectKindKey, *allocpb.AllocResponse)
	Delete(ObjectKindKey)
	ValidateExpiryTime(context.Context) map[ObjectKindKey]*allocpb.AllocResponse
}

func NewCache() Cache {
	return &cache{
		c: map[ObjectKindKey]*allocpb.AllocResponse{},
	}
}

// ObjectKindKey is the key of the Allocation, not the ownerKey
type ObjectKindKey struct {
	gvk schema.GroupVersionKind
	nsn types.NamespacedName
}

type cache struct {
	m sync.RWMutex
	c map[ObjectKindKey]*allocpb.AllocResponse
	l logr.Logger
}

func (r *cache) Get(key ObjectKindKey) *allocpb.AllocResponse {
	r.m.RLock()
	defer r.m.RUnlock()
	if allocRsp, ok := r.c[key]; ok {
		return allocRsp
	}
	return nil
}

func (r *cache) Add(key ObjectKindKey, allocRsp *allocpb.AllocResponse) {
	r.m.Lock()
	defer r.m.Unlock()
	r.c[key] = allocRsp
}

func (r *cache) Delete(key ObjectKindKey) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.c, key)
}

func getKey(alloc *allocpb.AllocRequest) ObjectKindKey {
	return ObjectKindKey{
		gvk: schema.GroupVersionKind{
			Group:   alloc.Header.Gvk.Group,
			Version: alloc.Header.Gvk.Version,
			Kind:    alloc.Header.Gvk.Kind,
		},
		nsn: types.NamespacedName{
			Namespace: alloc.Header.Nsn.Namespace,
			Name:      alloc.Header.Nsn.Name,
		},
	}
}

func isCacheDataValid(cacheResp *allocpb.AllocResponse, allocReq *allocpb.AllocRequest) bool {
	if cacheResp.Spec != allocReq.Spec {
		return false
	}
	if cacheResp.StatusCode != allocpb.StatusCode_Valid {
		return false
	}
	return true
}

func (r *cache) ValidateExpiryTime(ctx context.Context) map[ObjectKindKey]*allocpb.AllocResponse {
	r.l = log.FromContext(ctx)
	r.m.RLock()
	defer r.m.RUnlock()
	allocsToRefresh := map[ObjectKindKey]*allocpb.AllocResponse{}
	for key, allocpbResp := range r.c {
		if allocpbResp.ExpiryTime != "never" && allocpbResp.StatusCode == allocpb.StatusCode_Valid {
			t, err := time.Parse(time.RFC3339, allocpbResp.ExpiryTime)
			if err != nil {
				r.l.Error(err, "cannot unmarshal expirytime", "key", key)
				continue
			}
			minRefreshTime := time.Now().Add(time.Minute * 59)
			r.l.Info("expiry", "now", time.Now(), "expiryTime", t, "minRefreshTime", minRefreshTime, "delta", t.Sub(minRefreshTime), "key", key.gvk, "nsn", key.nsn)
			if t.Sub(minRefreshTime) < 0 {
				// add key to keys to be refreshed
				allocsToRefresh[key] = allocpbResp
			}
		}
	}
	return allocsToRefresh
}
