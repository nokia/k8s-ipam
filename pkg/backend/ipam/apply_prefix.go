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

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func NewApplicator(c *ApplicatorConfig) Applicator {
	return &applicator{
		initializing: c.initializing,
		claim:        c.claim,
		rib:          c.rib,
		pi:           c.pi,
		watcher:      c.watcher,
	}
}

type applicator struct {
	initializing bool
	claim        *ipamv1alpha1.IPClaim
	rib          *table.RIB
	pi           *iputil.Prefix
	watcher      Watcher
	l            logr.Logger
}

func (r *applicator) ApplyPrefix(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetName(), "kind", r.claim.Spec.Kind, "prefix", r.pi.GetIPPrefix().String(), "createPrefix", r.claim.Spec.CreatePrefix)
	r.l.Info("prefix claim")

	// get route
	var route table.Route
	var ok bool
	if r.claim.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
		if r.claim.Spec.CreatePrefix == nil {
			route, ok = r.rib.Get(r.pi.GetIPAddressPrefix())
		} else {
			route, ok = r.rib.Get(r.pi.GetIPSubnet())
		}
	} else {
		route, ok = r.rib.Get(r.pi.GetIPPrefix())
	}

	// if route exists -> update
	// if route does not exist -> add
	if ok {
		// prefix/route exists -> update
		r.l.Info("prefix claim: route exists", "route", route)

		// get all the routes from the routing table using the owner
		// to provide a common mechanism between dynamic claim and prefix claim
		routes, err := r.getRoutesByOwner()
		if err != nil {
			return err
		}
		if err := r.updateRib(ctx, routes); err != nil {
			return err
		}
	} else {
		r.l.Info("prefix claim: route does not exist", "route", route)

		// addRib mutates the routes and potentially expands the routes
		// add the routes to the RIB
		// update the status
		if err := r.addRib(ctx); err != nil {
			return err
		}
	}
	return nil
}
