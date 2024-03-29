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
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Validator interface {
	Validate(ctx context.Context) (string, error)
}

type validateInputFn func(claim *ipamv1alpha1.IPClaim, pi *iputil.Prefix) string
type validateChildrenExistFn func(route table.Route, prefixKind ipamv1alpha1.PrefixKind) string
type validateNoParentExistFn func(prefixKind ipamv1alpha1.PrefixKind, ownerGvk string) string
type validateParentExistFn func(route table.Route, claim *ipamv1alpha1.IPClaim, pi *iputil.Prefix) string

type PrefixValidatorFunctionConfig struct {
	validateInputFn         validateInputFn
	validateChildrenExistFn validateChildrenExistFn
	validateNoParentExistFn validateNoParentExistFn
	validateParentExistFn   validateParentExistFn
}

type PrefixValidatorConfig struct {
	claim *ipamv1alpha1.IPClaim
	rib   *table.RIB
	pi    *iputil.Prefix
	fnc   *PrefixValidatorFunctionConfig
}

func NewPrefixValidator(c *PrefixValidatorConfig) Validator {
	return &prefixvalidator{
		claim: c.claim,
		rib:   c.rib,
		pi:    c.pi,
		fnc:   c.fnc,
	}
}

type prefixvalidator struct {
	claim *ipamv1alpha1.IPClaim
	rib   *table.RIB
	pi    *iputil.Prefix
	fnc   *PrefixValidatorFunctionConfig
	l     logr.Logger
}

func (r *prefixvalidator) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("prefixkind", r.claim.Spec.Kind, "name", r.claim.GetName(), "prefix", r.claim.Spec.Prefix)
	r.l.Info("validate")

	r.l.Info("fnc", "r.fnc", r.fnc)

	// validate input
	if r.fnc.validateInputFn != nil {
		if msg := r.fnc.validateInputFn(r.claim, r.pi); msg != "" {
			return msg, nil
		}
	}

	// get dryrun rib
	dryrunRib := r.rib.Clone()

	// check if the prefix exists
	// for network based prefixes this is always the subnet (10.0.0.0/24) that is validated
	// for network based prefixes we need to do a second validation with the specific address
	// (10.0.0.x/24 which is turned into 10.0.0.x/32)
	route, ok := dryrunRib.Get(r.pi.GetIPSubnet())
	r.l.Info("validate if route exists",
		"createprefix", r.claim.Spec.CreatePrefix,
		"prefix", r.pi.GetIPPrefix(),
		"ok", ok,
		"route", route,
		"labels", r.claim.GetUserDefinedLabels())
	// Route exists
	if ok {
		if r.claim.Spec.CreatePrefix != nil {
			// this is a create/claim prefix:
			// ip prefix or a dynamic claim with a prefix
			if msg := validatePrefixOwner(route, r.claim); msg != "" {
				return msg, nil
			}
			// all good prefix exists and has the proper attributes
			return "", nil
		}
		// in case of a network prefix we need to turn this in an address and validate again
		// since the initial validation is for the network subnet and not for the individual address
		// in the subnet
		if r.claim.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
			// if parent is prefixkind network
			if route.Labels()[resourcev1alpha1.NephioPrefixKindKey] != string(ipamv1alpha1.PrefixKindNetwork) {
				return fmt.Sprintf("an address based prefix kind can only have parent prefix kind, got: %s", route.Labels()[resourcev1alpha1.NephioPrefixKindKey]), nil
			}
			route, ok = dryrunRib.Get(r.pi.GetIPAddressPrefix())
			if !ok {
				// we can return since we know there is a parent
				// the child is a /32 or /128 which cannot have children
				return "", nil
			}
		}
		if msg := validatePrefixOwner(route, r.claim); msg != "" {
			return msg, nil
		}
		// finish all good
		return "", nil
	}
	// Route does not exist
	// add dummy route in dry run rib, this is the subnet route
	// which serves as general purpose

	route = table.NewRoute(
		r.pi.GetIPSubnet(),
		r.claim.GetDummyLabelsFromPrefix(*r.pi),
		map[string]any{},
	)

	if err := dryrunRib.Add(route); err != nil {
		r.l.Error(err, "cannot add route", "route", route)
		return "", err
	}

	// get the route again and check for children
	route, ok = dryrunRib.Get(r.pi.GetIPSubnet())
	if !ok {
		return "route just added, but a new get does not find it", nil
	}
	// check for children
	routes := route.Children(dryrunRib)
	if len(routes) > 0 {
		r.l.Info("got children", "routes", routes)
		if msg := r.fnc.validateChildrenExistFn(routes[0], r.claim.Spec.Kind); msg != "" {
			return msg, nil
		}
	}
	// get parents

	routes = route.Parents(dryrunRib)
	if len(routes) == 0 {
		if msg := r.fnc.validateNoParentExistFn(r.claim.Spec.Kind, r.claim.GetUserDefinedLabels()[resourcev1alpha1.NephioOwnerGvkKey]); msg != "" {
			return msg, nil
		}
		return "", nil
	}

	parentRoute := findParent(routes)
	if msg := r.fnc.validateParentExistFn(parentRoute, r.claim, r.pi); msg != "" {
		return msg, nil
	}
	return "", nil
}

func findParent(routes table.Routes) table.Route {
	parentRoute := routes[0]
	for _, route := range routes {
		if route.Prefix().Bits() > parentRoute.Prefix().Bits() {
			parentRoute = route
		}
	}
	return parentRoute
}
