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
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewPrefixRuntime(cfg any) (Runtime, error) {
	c, ok := cfg.(*PrefixRuntimeConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config expecting IPAMPrefixOperatorConfig")
	}
	pi, err := iputil.New(*c.claim.Spec.Prefix)
	if err != nil {
		return nil, err
	}
	return &prefixRuntime{
		initializing: c.initializing,
		claim:        c.claim,
		rib:          c.rib,
		fnc:          c.fnc,
		pi:           pi,
		watcher:      c.watcher,
	}, nil
}

type prefixRuntime struct {
	initializing bool
	claim        *ipamv1alpha1.IPClaim
	rib          *table.RIB
	pi           *iputil.Prefix
	fnc          *PrefixValidatorFunctionConfig
	watcher      Watcher
	l            logr.Logger
}

func (r *prefixRuntime) Get(ctx context.Context) (*ipamv1alpha1.IPClaim, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetGenericNamespacedName(), "prefixkind", r.claim.Spec.Kind, "prefix", r.claim.Spec.Prefix)
	r.l.Info("get")
	g := NewGetter(&GetterConfig{
		claim: r.claim,
		rib:   r.rib,
	})
	if err := g.GetIPClaim(ctx); err != nil {
		return nil, err
	}

	r.l.Info("get claim done", "status", r.claim.Status)
	return r.claim, nil
}

func (r *prefixRuntime) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetGenericNamespacedName(), "prefixkind", r.claim.Spec.Kind, "prefix", r.claim.Spec.Prefix)
	r.l.Info("validate")
	v := NewPrefixValidator(&PrefixValidatorConfig{
		claim: r.claim,
		rib:   r.rib,
		pi:    r.pi,
		fnc:   r.fnc,
	})
	return v.Validate(ctx)
}

func (r *prefixRuntime) Apply(ctx context.Context) (*ipamv1alpha1.IPClaim, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetName(), "prefixkind", r.claim.Spec.Kind, "prefix", r.claim.Spec.Prefix)
	r.l.Info("apply")

	pi, err := iputil.New(*r.claim.Spec.Prefix)
	if err != nil {
		return nil, err
	}
	a := NewApplicator(&ApplicatorConfig{
		initializing: r.initializing,
		claim:        r.claim,
		rib:          r.rib,
		pi:           pi,
		watcher:      r.watcher,
	})
	if err := a.ApplyPrefix(ctx); err != nil {
		return nil, err
	}

	r.l.Info("claimed prefix done", "status", r.claim.Status)
	return r.claim, nil

}
func (r *prefixRuntime) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetName(), "prefixkind", r.claim.Spec.Kind, "prefix", r.claim.Spec.Prefix)
	r.l.Info("delete")

	r.l.Info("delete prefix claim", "claim", r.claim)
	d := NewApplicator(&ApplicatorConfig{
		initializing: r.initializing,
		claim:        r.claim,
		rib:          r.rib,
		watcher:      r.watcher,
	})
	if err := d.Delete(ctx); err != nil {
		return err
	}

	return nil
}
