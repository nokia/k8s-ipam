package proxycache

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/henderiw-k8s-lcnc/discovery/registrator"
	"github.com/nokia/k8s-ipam/pkg/alloc/alloc"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type ProxyCache interface {
	// Discovers the server
	Start(context.Context)
	// add the event channels to the proxy cache
	AddEventChs(map[schema.GroupVersionKind]chan event.GenericEvent)
	// registers a response validator to validate the content of the response
	// given the proxy cache is content agnostic it needs a helper to execute this task
	RegisterRefreshRespValidator(key string, fn RefreshRespValidatorFn)
	// Get returns the allocated prefix
	Get(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error)
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
		informer:    NewNopInformer(),
		cache:       NewCache(),
		validator:   NewResponseValidator(),
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
	watchCancel context.CancelFunc
	// validator
	validator ResponseValidator
	//logger
	l logr.Logger
}

// AddEventChs add the ownerGvk's event channels to the informaer
func (r *proxycache) AddEventChs(eventChannels map[schema.GroupVersionKind]chan event.GenericEvent) {
	r.informer = NewInformer(eventChannels)

}

func (r *proxycache) RegisterRefreshRespValidator(key string, fn RefreshRespValidatorFn) {
	r.validator.Add(key, fn)
}

func (r *proxycache) Start(ctx context.Context) {
	r.l.Info("starting proxy cache")
	ch := r.registrator.Watch(ctx, "ipam", []string{}, registrator.WatchOptions{RetriveServices: true})
	// this is used to control the watch go routine
	watchCtx, cancel := context.WithCancel(ctx)
	r.watchCancel = cancel

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

							continue GeneralWait
						}
					}
				}
				if len(svcInfo.ServiceInstances) == 0 {
					r.l.Info("service, no available service -> delete Client")
					// delete client
					if err := r.deleteClient(watchCtx); err != nil {
						r.l.Error(err, "cannot delete client")
					}
					continue GeneralWait
				}
				r.svcInfo = svcInfo.ServiceInstances[0]
				r.l.Info("service, info changed-> delete and create Client", "svcInfo", svcInfo.ServiceInstances[0])
				// delete client
				if err := r.deleteClient(watchCtx); err != nil {
					r.l.Error(err, "cannot delete client")
					continue GeneralWait
				}
				// create client
				if err := r.createClient(watchCtx); err != nil {
					r.l.Error(err, "cannot create client")
				}
			case <-ctx.Done():
				// called when the controller gets cancelled
				return
			case <-time.After(time.Second * 5):
				// walk cache and check the expiry time
				r.l.Info("refresh handler")
				keysToRefresh := r.cache.ValidateExpiryTime(ctx)
				var wg sync.WaitGroup
				for objKey, allocpbResp := range keysToRefresh {
					r.l.Info("refresh allocation", "gvk", objKey.gvk, "nsn", objKey.nsn)
					wg.Add(1)
					t := time.Now().Add(time.Minute * 60)
					b, err := t.MarshalText()
					if err != nil {
						r.l.Error(err, "cannot marshal the time during refresh")
					}
					req := &allocpb.Request{
						Header:     allocpbResp.GetHeader(),
						Spec:       allocpbResp.GetSpec(),
						ExpiryTime: string(b)}

					ownerGvk := schema.GroupVersionKind{
						Group:   allocpbResp.GetHeader().GetOwnerGvk().GetGroup(),
						Version: allocpbResp.GetHeader().GetOwnerGvk().GetVersion(),
						Kind:    allocpbResp.GetHeader().GetOwnerGvk().GetKind(),
					}
					ownerNsn := types.NamespacedName{
						Namespace: allocpbResp.GetHeader().GetOwnerNsn().GetNamespace(),
						Name:      allocpbResp.GetHeader().GetOwnerNsn().GetName(),
					}
					group := allocpbResp.GetHeader().GetGvk().GetGroup()
					origresp := allocpbResp
					objKey := objKey

					go func() {
						defer wg.Done()

						// refresh the allocation
						resp, err := r.refreshAllocate(ctx, req)
						if err != nil {
							// if we get an error in the response, log it and inform the client
							r.l.Error(err, "refresh allocation failed")
							// remove the cache entry
							r.cache.Delete(objKey)
							r.informer.NotifyClient(ownerGvk, ownerNsn)
						}
						// TBD if we need more protection
						if resp != nil {
							r.l.Info("refresh resp", "resp", resp.Status)
							// Validate the response through the client proxy registered validator
							// if the validator is not happy with the response we notify the client
							if r.validator.Get(group) != nil {
								if !r.validator.Get(group)(origresp, resp) {
									r.l.Error(err, "refresh validation NOK")
									// remove the cache entry
									r.cache.Delete(objKey)
									r.informer.NotifyClient(ownerGvk, ownerNsn)
								}
							}
						}

					}()
				}
				wg.Wait()
			}
		}
	}()
}

func (r *proxycache) Get(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	allocClient, err := r.getClient()
	if err != nil {
		return nil, err
	}
	// TBD if we need to use the cache here
	allocResp, err := allocClient.Get(ctx, alloc)
	if err != nil || allocResp.GetStatusCode() != allocpb.StatusCode_Valid {
		return allocResp, err
	}
	return allocResp, err
}

func (r *proxycache) Allocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	return r.allocate(ctx, alloc, false)
}

func (r *proxycache) refreshAllocate(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	return r.allocate(ctx, alloc, true)
}

// refresh flag indicates if the allocation is initiated for a refresh
func (r *proxycache) allocate(ctx context.Context, alloc *allocpb.Request, refresh bool) (*allocpb.Response, error) {
	allocClient, err := r.getClient()
	if err != nil {
		return nil, err
	}

	key := getKey(alloc)
	if !refresh {
		cacheData := r.cache.Get(key)
		if cacheData != nil {
			// check if the data is available and consistent
			if isCacheDataValid(cacheData, alloc) {
				r.l.Info("cache hit OK -> response from cache", "keyGVK", key.gvk, "keyNsn", key.nsn)
				return cacheData, nil
			}
		}
	}
	if refresh {
		r.l.Info("cache hit NOK -> refresh from ipam server", "keyGVK", key.gvk, "keyNsn", key.nsn)
	} else {
		// allocate the prefix from the central ipam server
		r.l.Info("cache hit NOK -> response from ipam server", "keyGVK", key.gvk, "keyNsn", key.nsn)
	}

	allocResp, err := allocClient.Allocate(ctx, alloc)
	if err != nil || allocResp.GetStatusCode() != allocpb.StatusCode_Valid {
		return allocResp, err
	}
	// if the allocation is successfull we add the entry in the cache
	r.cache.Add(key, allocResp)
	return allocResp, err
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
