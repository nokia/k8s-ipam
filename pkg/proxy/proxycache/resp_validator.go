package proxycache

import (
	"sync"

	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
)

type RefreshRespValidatorFn func(origResp *allocpb.AllocResponse, newResp *allocpb.AllocResponse) bool

type ResponseValidator interface {
	Add(string, RefreshRespValidatorFn)
	Get(string) RefreshRespValidatorFn
}

func NewResponseValidator() ResponseValidator {
	return &respValidator{
		v: map[string]RefreshRespValidatorFn{},
	}
}

type respValidator struct {
	m sync.RWMutex
	v map[string]RefreshRespValidatorFn
}

func (r *respValidator) Add(key string, fn RefreshRespValidatorFn) {
	r.m.Lock()
	defer r.m.Unlock()
	r.v[key] = fn
}

func (r *respValidator) Get(key string) RefreshRespValidatorFn {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.v[key]
}
