package proxycache

import (
	"sync"

	"github.com/nokia/k8s-ipam/internal/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// informer works based on the owner GVK
type Informer interface {
	Add(schema.GroupVersionKind, chan event.GenericEvent)
	Delete(schema.GroupVersionKind)
	NotifyClient(gvk schema.GroupVersionKind, namespace, name string)
}

func NewInformer(EventChannels map[schema.GroupVersionKind]chan event.GenericEvent) Informer {
	return &informer{
		eventCh: EventChannels,
	}
}

type informer struct {
	m sync.RWMutex
	// gvk key is the origin gvk
	eventCh map[schema.GroupVersionKind]chan event.GenericEvent
}

func (r *informer) Add(gvk schema.GroupVersionKind, ch chan event.GenericEvent) {
	r.m.Lock()
	defer r.m.Unlock()
	r.eventCh[gvk] = ch
}

func (r *informer) Delete(gvk schema.GroupVersionKind) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.eventCh, gvk)
}

func (r *informer) NotifyClient(gvk schema.GroupVersionKind, namespace, name string) {
	r.m.RLock()
	defer r.m.RUnlock()

	u := meta.GetUnstructuredFromGVK(&gvk)
	u.SetName(name)
	u.SetNamespace(namespace)

	if eventCh, ok := r.eventCh[gvk]; ok {
		eventCh <- event.GenericEvent{
			Object: u,
		}
	}
}
