package ipam

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewPrefixRuntime(cfg any) (Runtime, error) {
	c, ok := cfg.(*PrefixRuntimeConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config expecting IPAMPrefixOperatorConfig")
	}
	pi, err := iputil.New(c.alloc.GetPrefix())
	if err != nil {
		return nil, err
	}
	return &prefixRuntime{
		initializing: c.initializing,
		alloc:        c.alloc,
		rib:          c.rib,
		fnc:          c.fnc,
		pi:           pi,
		watcher:      c.watcher,
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

func (r *prefixRuntime) Get(ctx context.Context) (*ipamv1alpha1.IPAllocation, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetGenericNamespacedName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
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

	r.l.Info("allocate prefix done", "status", r.alloc.Status)
	return r.alloc, nil

}
func (r *prefixRuntime) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.alloc.GetName(), "prefixkind", r.alloc.GetPrefixKind(), "prefix", r.alloc.GetPrefix())
	r.l.Info("delete")

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
