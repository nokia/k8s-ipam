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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewDynamicRuntime(cfg any) (Runtime, error) {
	c, ok := cfg.(*DynamicRuntimeConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config expecting DynamicRuntimeConfig")
	}
	return &claimRuntime{
		initializing: c.initializing,
		claim:        c.claim,
		rib:          c.rib,
		fnc:          c.fnc,
		watcher:      c.watcher,
	}, nil
}

type claimRuntime struct {
	initializing bool
	claim        *ipamv1alpha1.IPClaim
	rib          *table.RIB
	fnc          *DynamicValidatorFunctionConfig
	watcher      Watcher
	l            logr.Logger
}

func (r *claimRuntime) Get(ctx context.Context) (*ipamv1alpha1.IPClaim, error) {
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

func (r *claimRuntime) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetGenericNamespacedName(), "prefixkind", r.claim.Spec.Kind, "prefix", r.claim.Spec.Prefix)
	r.l.Info("validate")
	v := NewClaimValidator(&ClaimValidatorConfig{
		claim: r.claim,
		rib:   r.rib,
		fnc:   r.fnc,
	})
	return v.Validate(ctx)
}

func (r *claimRuntime) Apply(ctx context.Context) (*ipamv1alpha1.IPClaim, error) {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetGenericNamespacedName(), "prefixkind", r.claim.Spec.Kind, "prefix", r.claim.Spec.Prefix)
	r.l.Info("apply dynamic claim")

	a := NewApplicator(&ApplicatorConfig{
		initializing: r.initializing,
		claim:        r.claim,
		rib:          r.rib,
		watcher:      r.watcher,
	})
	if err := a.ApplyDynamic(ctx); err != nil {
		r.l.Error(err, "cannot apply dynamic claim")
		return nil, err
	}
	r.l.Info("apply dynamic prefix done done", "status", r.claim.Status)
	return r.claim, nil

}
func (r *claimRuntime) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetName(), "prefixkind", r.claim.Spec.Kind, "prefix", r.claim.Spec.Prefix)
	r.l.Info("delete")

	r.l.Info("delete dynamic claim", "claim", r.claim)
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
