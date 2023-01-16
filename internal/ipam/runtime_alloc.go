package ipam

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewAllocOperator(cfg any) (Runtime, error) {
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

func (r *allocRuntime) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("validate")
	v := NewAllocValidator(&AllocValidatorConfig{
		alloc: r.alloc,
		rib:   r.rib,
		fnc:   r.fnc,
	})
	return v.Validate(ctx)
}

func (r *allocRuntime) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("apply")

	allocs := r.getMutatedAllocs(ctx)
	var updatedAlloc *ipamv1alpha1.IPAllocation
	for _, alloc := range allocs {
		a := NewAllocApplicator(&ApplicatorConfig{
			initializing: r.initializing,
			alloc:   alloc,
			rib:     r.rib,
			watcher: r.watcher,
		})
		ap, err := a.Apply(ctx)
		if err != nil {
			return nil, err
		}
		r.l.Info("allocate prefix", "name", alloc.GetName(), "prefix", alloc.GetPrefix())
		if r.alloc.GetName() == alloc.GetName() {
			updatedAlloc = ap
		}
	}
	r.l.Info("allocate prefix done", "updatedAlloc", updatedAlloc)
	return updatedAlloc, nil

}
func (r *allocRuntime) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("delete")

	allocs := r.getMutatedAllocs(ctx)
	for _, alloc := range allocs {
		r.l.Info("deallocate individual prefix", "alloc", alloc)
		d := NewDeleteApplicator(&ApplicatorConfig{
			initializing: r.initializing,
			alloc:        alloc,
			rib:          r.rib,
			watcher:      r.watcher,
		})
		if err := d.Delete(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *allocRuntime) getMutatedAllocs(ctx context.Context) []*ipamv1alpha1.IPAllocation {
	m := NewMutator(&MutatorConfig{
		alloc: r.alloc,
	})
	return m.MutateAllocWithoutPrefix(ctx)
}
