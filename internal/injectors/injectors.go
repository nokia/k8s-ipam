/*
Copyright 2022 Nokia.

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

package injectors

import (
	"context"
	"sync"

	"github.com/nokia/k8s-ipam/internal/injector"
)

type Injectors interface {
	Run(injector.Injector)
	Stop(injector.Injector)
}

type injectors struct {
	m         sync.Mutex
	injectors map[string]injector.Injector
}

func New() Injectors {
	return &injectors{
		injectors: make(map[string]injector.Injector),
	}
}

func (r *injectors) Run(i injector.Injector) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.injectors[i.GetName()]; !ok {
		r.injectors[i.GetName()] = i
		go r.injectors[i.GetName()].Run(context.Background())
	}

}

func (r *injectors) Stop(i injector.Injector) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.injectors[i.GetName()]; ok {
		r.injectors[i.GetName()].Stop()
	}
	delete(r.injectors, i.GetName())
}
