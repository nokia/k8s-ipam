package proxycache

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/internal/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// informer works based on the owner GVK
type Informer interface {
	Add(schema.GroupVersionKind, chan event.GenericEvent)
	Delete(schema.GroupVersionKind)
	NotifyClient(schema.GroupVersionKind, types.NamespacedName)
	GetGVK() []schema.GroupVersionKind
}

func NewNopInformer() Informer {
	l := ctrl.Log.WithName("nopInformer")
	return &informer{
		eventCh: map[schema.GroupVersionKind]chan event.GenericEvent{},
		l:       l,
	}
}

func NewInformer(EventChannels map[schema.GroupVersionKind]chan event.GenericEvent) Informer {
	l := ctrl.Log.WithName("informer")
	return &informer{
		eventCh: EventChannels,
		l:       l,
	}
}

type informer struct {
	m sync.RWMutex
	// gvk key is the origin gvk
	eventCh map[schema.GroupVersionKind]chan event.GenericEvent
	// logger
	l logr.Logger
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

func (r *informer) NotifyClient(ownerGvk schema.GroupVersionKind, ownerNsn types.NamespacedName) {
	r.m.RLock()
	defer r.m.RUnlock()

	u := meta.GetUnstructuredFromGVK(&ownerGvk)
	u.SetName(ownerNsn.Name)
	u.SetNamespace(ownerNsn.Namespace)

	if eventCh, ok := r.eventCh[ownerGvk]; ok {
		r.l.Info("notifyClient", "gvk", ownerGvk, "nsn", ownerNsn, "obj", u)
		eventCh <- event.GenericEvent{
			Object: u,
		}
	} else {
		r.l.Info("notifyClient gvk not found", "gvk", ownerGvk, "nsn", ownerNsn, "obj", u)
	}
}

func (r *informer) GetGVK() []schema.GroupVersionKind {
	r.m.RLock()
	defer r.m.RUnlock()
	gvks := make([]schema.GroupVersionKind, 0, len(r.eventCh))
	for gvk := range r.eventCh {
		gvks = append(gvks, gvk)
	}
	return gvks
}
