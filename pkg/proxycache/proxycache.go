package proxycache

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/go-logr/logr"
	"github.com/henderiw-k8s-lcnc/discovery/registrator"
	"github.com/nokia/k8s-ipam/pkg/alloc/alloc"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type ProxyCache interface {
	// Discovers the server
	Start(context.Context)
	// add the event channels to the proxy cache
	AddEventChs(map[schema.GroupVersionKind]chan event.GenericEvent)
	// Allocate -> lookup in local cache based on (ipam, etc) gvknsn
	Allocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error)
	// DeAllocate removes the cache data
	DeAllocate(ctx context.Context, alloc *allocpb.Request) error
	// Timer based refresh Config/State Cache
	// NotifyClient controller though generic event (informer) -> all the For
}

type Config struct {
	Registrator registrator.Registrator
	//EventChannels map[schema.GroupVersionKind]chan event.GenericEvent
}

func New(c *Config) ProxyCache {
	l := ctrl.Log.WithName("proxy-cache")

	return &proxycache{
		//informer:    NewInformer(c.EventChannels),
		cache:       NewCache(),
		registrator: c.Registrator,
		l:           l,
	}
}

type proxycache struct {
	informer Informer
	// this is the ipam GVK namespace, name
	cache Cache
	// registrator finds the ipam
	registrator registrator.Registrator
	svcInfo     *registrator.Service
	m           sync.RWMutex
	allocClient alloc.Client
	//logger
	l logr.Logger
}

func (r *proxycache) AddEventChs(eventChannels map[schema.GroupVersionKind]chan event.GenericEvent) {
	r.informer = NewInformer(eventChannels)
}

func (r *proxycache) Start(ctx context.Context) {
	r.l.Info("starting proxy cache")
	ch := r.registrator.Watch(ctx, "ipam", []string{}, registrator.WatchOptions{RetriveServices: true})

	go func() {
	GeneralWait:
		for {
			select {
			case svcInfo := <-ch:
				r.l.Info("service", "info", *svcInfo)

				if r.svcInfo != nil {
					for _, service := range svcInfo.ServiceInstances {
						if service.Address == r.svcInfo.Address &&
							service.Port == r.svcInfo.Port &&
							service.ID == r.svcInfo.ID &&
							service.Name == r.svcInfo.Name {
							r.l.Info("service, no change, keep waiting", "allocClient", r.allocClient.Get())

							/*
								allocpbReq, _ := ipamalloc.BuildAllocationFromIPAllocation(buildDummyIPAllocation(), time.Now().UTC().Add(time.Minute*60).String())
								resp, err := r.allocClient.Get().Allocation(context.TODO(), allocpbReq)
								if err != nil {
									r.l.Error(err, "cannot get allocation§")
								} else {
									r.l.Info("service, no change, keep waiting", "resp", resp)
								}
							*/

							continue GeneralWait
						}
					}
				}
				if len(svcInfo.ServiceInstances) == 0 {
					r.l.Info("service, no available service -> delete Client")
					// delete client
					r.m.Lock()
					if r.allocClient != nil {
						if err := r.allocClient.Delete(); err != nil {
							r.l.Error(err, "cannot delete client")
						}
						continue GeneralWait
					}
					r.allocClient = nil
					r.m.Unlock()

				} else {
					r.svcInfo = svcInfo.ServiceInstances[0]
					r.l.Info("service, info changed-> delete and create Client", "svcInfo", svcInfo.ServiceInstances[0])
					r.m.Lock()
					// delete client
					if r.allocClient != nil {
						if err := r.allocClient.Delete(); err != nil {
							r.l.Error(err, "cannot delete client")
						}
						continue GeneralWait
					}
					// create client
					ac, err := alloc.New(&alloc.Config{
						Address:  fmt.Sprintf("%s:%s", r.svcInfo.Address, strconv.Itoa(r.svcInfo.Port)),
						Insecure: true,
					})
					if err != nil {
						r.l.Error(err, "cannot create client")
						r.allocClient = nil
						continue GeneralWait
					}
					r.allocClient = ac
					r.m.Unlock()
				}
			case <-ctx.Done():
				// called when the controller gets cancelled
				return
			}
		}
	}()
}

func (r *proxycache) Allocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	key := getKey(alloc)

	allocClient, err := r.getClient()
	if err != nil {
		return nil, err
	}

	cacheData := r.cache.Get(key)
	if cacheData != nil {
		// check if the data is available and consistent
		if isCacheDataValid(cacheData, alloc) {
			r.l.Info("cache hit, response from cache")
			return cacheData, nil
		}
	}
	// allocate the prefix from the central ipam server
	r.l.Info("no cache hit, response from central ipam server")
	return allocClient.Allocate(ctx, alloc)
}

func (r *proxycache) DeAllocate(ctx context.Context, alloc *allocpb.Request) error {
	allocClient, err := r.getClient()
	if err != nil {
		return err
	}
	_, err = allocClient.DeAllocate(ctx, alloc)
	if err != nil {
		return err
	}
	// delete the cache only if the DeAllocation is successfull
	r.cache.Delete(getKey(alloc))
	return nil

}

func (r *proxycache) getClient() (allocpb.AllocationClient, error) {
	r.m.RLock()
	defer r.m.RUnlock()
	if r.allocClient == nil {
		return nil, fmt.Errorf("ipam server unreachable")
	}
	return r.allocClient.Get(), nil
}

/*
func buildDummyIPAllocation() *ipamv1alpha1.IPAllocation {
	return &ipamv1alpha1.IPAllocation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ipamv1alpha1.IPAllocationKindAPIVersion,
			Kind:       ipamv1alpha1.IPAllocationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "dummyAlloc",
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			PrefixKind:      ipamv1alpha1.PrefixKindLoopback,
			NetworkInstance: "vpc-mgmt2",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"nephio.org/fabric":  "fabric1",
					"nephio.org/purpose": "mgmt",
				},
			},
		},
	}
}
*/
