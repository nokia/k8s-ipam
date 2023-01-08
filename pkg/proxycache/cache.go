package proxycache

import (
	"sync"

	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type Cache interface {
	Get(ObjectKindKey) *allocpb.Response
	Add(ObjectKindKey, *allocpb.Response)
	Delete(ObjectKindKey)
	// TODO timer function
}

func NewCache() Cache {
	return &cache{
		c: map[ObjectKindKey]*cacheData{},
	}
}

// ObjectKindKey is the key of the Allocation, not the ownerKey
type ObjectKindKey struct {
	gvk schema.GroupVersionKind
	nsn types.NamespacedName
}

type cache struct {
	m sync.RWMutex
	c map[ObjectKindKey]*cacheData
}

type cacheData struct {
	data *allocpb.Response
	// timer per entry ??
}

func (r *cache) Get(key ObjectKindKey) *allocpb.Response {
	r.m.RLock()
	defer r.m.RUnlock()
	if cd, ok := r.c[key]; ok {
		return cd.data
	}
	return nil
}

func (r *cache) Add(key ObjectKindKey, d *allocpb.Response) {
	r.m.Lock()
	defer r.m.Unlock()
	r.c[key] = &cacheData{
		data: d,
	}
}

func (r *cache) Delete(key ObjectKindKey) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.c, key)
}

func getKey(alloc *allocpb.Request) ObjectKindKey {
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

func isCacheDataValid(cacheResp *allocpb.Response, allocReq *allocpb.Request) bool {
	if cacheResp.Spec != allocReq.Spec {
		return false
	}
	if cacheResp.StatusCode != allocpb.StatusCode_Valid {
		return false
	}
	return true
}
