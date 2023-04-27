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

package ipam

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewAllocRuntime(cfg any) (Runtime, error) {
	c, ok := cfg.(*AllocRuntimeConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config expecting IPAMAllocOperatorConfig")
	}
	fmt.Printf("NewAllocOperator: %v\n", *c.fnc)
	return &allocRuntime{
		initializing: c.initializing,
		alloc:        c.alloc,
		rib:          c.rib,
		fnc:          c.fnc,
		watcher:      c.watcher,
	}, nil
}

type allocRuntime struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	fnc          *AllocValidatorFunctionConfig
	watcher      Watcher
	l            logr.Logger
}

func (r *allocRuntime) Get(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.Spec.Kind, "prefix", r.alloc.Spec.Prefix)
	r.l.Info("get")
	g := NewGetter(&GetterConfig{
		alloc: r.alloc,
		rib:   r.rib,
	})
	if err := g.GetIPAllocation(ctx); err != nil {
		return nil, err
	}

	r.l.Info("get allocation done", "status", r.alloc.Status)
	return r.alloc, nil
}

func (r *allocRuntime) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.Spec.Kind, "prefix",  r.alloc.Spec.Prefix)
	r.l.Info("validate")
	v := NewAllocValidator(&AllocValidatorConfig{
		alloc: r.alloc,
		rib:   r.rib,
		fnc:   r.fnc,
	})
	return v.Validate(ctx)
}

func (r *allocRuntime) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.Spec.Kind, "prefix",  r.alloc.Spec.Prefix)
	r.l.Info("apply dynamic allocation")

	a := NewApplicator(&ApplicatorConfig{
		initializing: r.initializing,
		alloc:        r.alloc,
		rib:          r.rib,
		watcher:      r.watcher,
	})
	if err := a.ApplyAlloc(ctx); err != nil {
		return nil, err
	}
	r.l.Info("allocate dynamic allocation done done", "status", r.alloc.Status)
	return r.alloc, nil

}
func (r *allocRuntime) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "prefixkind", r.alloc.Spec.Kind, "prefix", r.alloc.Spec.Prefix)
	r.l.Info("delete")

	r.l.Info("deallocate dynamic allocation", "alloc", r.alloc)
	d := NewApplicator(&ApplicatorConfig{
		initializing: r.initializing,
		alloc:        r.alloc,
		rib:          r.rib,
		watcher:      r.watcher,
	})
	if err := d.Delete(ctx); err != nil {
		return err
	}
	return nil
}
