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
	"sync"

	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
)

type RefreshRespValidatorFn func(origResp *allocpb.AllocResponse, newResp *allocpb.AllocResponse) bool

// validates the response from the grpc server with the specific validate fn
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