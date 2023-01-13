package ipam

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewAllocOperator(cfg any) (IPAMOperation, error) {
	c, ok := cfg.(*IPAMAllocOperatorConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config expecting IPAMAllocOperatorConfig")
	}
	return &allocOperator{
		alloc: c.alloc,
		rib:   c.rib,
		fnc:   c.fnc,
	}, nil
}

type allocOperator struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	fnc   *AllocValidatorFunctionConfig
	l     logr.Logger
}

func (r *allocOperator) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("validate")
	v := NewAllocValidator(&AllocValidatorConfig{
		alloc: r.alloc,
		rib:   r.rib,
		fnc:   r.fnc,
	})
	return v.Validate(ctx)
}

func (r *allocOperator) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("apply")

	allocs := r.getMutatedAllocs(ctx)
	var updatedAlloc *ipamv1alpha1.IPAllocation
	for _, alloc := range allocs {
		r.l.Info("allocate individual prefix", "alloc", alloc)
		pi, err := iputil.New(alloc.GetPrefix())
		if err != nil {
			return nil, err
		}
		a := NewAllocApplicator(&ApplicatorConfig{
			alloc: alloc,
			rib:   r.rib,
			pi:    pi,
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
func (r *allocOperator) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("delete")

	allocs := r.getMutatedAllocs(ctx)
	for _, alloc := range allocs {
		r.l.Info("deallocate individual prefix", "alloc", alloc)
		d := NewDeleteApplicator(&ApplicatorConfig{
			alloc: alloc,
			rib:   r.rib,
		})
		if err := d.Delete(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *allocOperator) getMutatedAllocs(ctx context.Context) []*ipamv1alpha1.IPAllocation {
	m := NewMutator(&MutatorConfig{
		alloc: r.alloc,
	})
	return m.MutateAllocWithoutPrefix(ctx)
}
