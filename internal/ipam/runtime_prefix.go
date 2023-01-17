package ipam

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewPrefixRuntime(cfg *PrefixRuntimeConfig) (Runtime, error) {
	pi, err := iputil.New(cfg.alloc.GetPrefix())
	if err != nil {
		return nil, err
	}
	return &prefixRuntime{
		initializing: cfg.initializing,
		alloc:        cfg.alloc,
		rib:          cfg.rib,
		fnc:          cfg.fnc,
		pi:           pi,
		watcher:      cfg.watcher,
	}, nil
}

type prefixRuntime struct {
	initializing bool
	alloc        *ipamv1alpha1.IPAllocation
	rib          *table.RIB
	pi           iputil.PrefixInfo
	fnc          *PrefixValidatorFunctionConfig
	watcher      Watcher
	l            logr.Logger
}

func (r *prefixRuntime) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("validate")
	v := NewPrefixValidator(&PrefixValidatorConfig{
		alloc: r.alloc,
		rib:   r.rib,
		pi:    r.pi,
		fnc:   r.fnc,
	})
	return v.Validate(ctx)
}

func (r *prefixRuntime) Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("apply")

	//allocs := r.getMutatedAllocs(ctx)
	//var updatedAlloc *ipamv1alpha1.IPAllocation
	//for _, alloc := range allocs {
	//r.l.Info("allocate individual prefix", "alloc", alloc)
	pi, err := iputil.New(r.alloc.GetPrefix())
	if err != nil {
		return nil, err
	}
	a := NewApplicator(&ApplicatorConfig{
		initializing: r.initializing,
		alloc:        r.alloc,
		rib:          r.rib,
		pi:           pi,
		watcher:      r.watcher,
	})
	if err := a.ApplyPrefix(ctx); err != nil {
		return nil, err
	}
	//r.l.Info("allocate prefix", "name", r.alloc.GetName(), "prefix", r.alloc.GetPrefix())
	//if r.alloc.GetName() == alloc.GetName() {
	//	updatedAlloc = ap
	//}
	//}
	r.l.Info("allocate prefix done", "status", r.alloc.Status)
	return r.alloc, nil

}
func (r *prefixRuntime) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("delete")

	//allocs := r.getMutatedAllocs(ctx)
	//for _, alloc := range allocs {
	r.l.Info("deallocate prefix allocation", "alloc", r.alloc)
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

/*
func (r *prefixRuntime) getMutatedAllocs(ctx context.Context) []*ipamv1alpha1.IPAllocation {
	m := NewMutator(&MutatorConfig{
		alloc: r.alloc,
		pi:    r.pi,
	})
	if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork && r.alloc.GetCreatePrefix() {
		return m.MutateAllocNetworkWithPrefix(ctx)
	}
	return m.MutateAllocWithPrefix(ctx)
}
*/
