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

type Validator interface {
	Validate(ctx context.Context) (string, error)
}

type validateInputFn func(alloc *ipamv1alpha1.IPAllocation, pi iputil.PrefixInfo) string
type validateChildrenExistFn func(route table.Route, prefixKind ipamv1alpha1.PrefixKind) string
type validateNoParentExistFn func(prefixKind ipamv1alpha1.PrefixKind, ownerGvk string) string
type validateParentExistFn func(route table.Route, prefixKind ipamv1alpha1.PrefixKind, pi iputil.PrefixInfo) string

type PrefixValidatorFunctionConfig struct {
	validateInputFn         validateInputFn
	validateChildrenExistFn validateChildrenExistFn
	validateNoParentExistFn validateNoParentExistFn
	validateParentExistFn   validateParentExistFn
}

type PrefixValidatorConfig struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	pi    iputil.PrefixInfo
	fnc   *PrefixValidatorFunctionConfig
}

func NewPrefixValidator(c *PrefixValidatorConfig) Validator {
	return &prefixvalidator{
		alloc: c.alloc,
		rib:   c.rib,
		pi:    c.pi,
		fnc:   c.fnc,
	}
}

type prefixvalidator struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	pi    iputil.PrefixInfo
	fnc   *PrefixValidatorFunctionConfig
	l     logr.Logger
}

func (r *prefixvalidator) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("prefixkind", r.alloc.GetPrefixKind(), "name", r.alloc.GetName(), "prefix", r.alloc.GetPrefix())
	r.l.Info("validate")

	// validate input
	if msg := r.fnc.validateInputFn(r.alloc, r.pi); msg != "" {
		return msg, nil
	}

	// get dryrun rib
	dryrunRib := r.rib.Clone()

	// check if the prefix exists
	// for network based prefixes this is always the subnet (10.0.0.0/24) that is validated
	// for network based prefixes we need to do a second validation with the specific address
	// (10.0.0.x/24 which is turned into 10.0.0.x/32)
	route, ok := dryrunRib.Get(r.pi.GetIPSubnet())
	r.l.Info("validate if route exists",
		"createprefix", r.alloc.GetCreatePrefix(),
		"prefix", r.pi.GetIPPrefix(),
		"ok", ok,
		"route", route,
		"labels", r.alloc.GetSpecLabels())
	// Route exists
	if ok {
		if r.alloc.GetCreatePrefix() {
			// this is a create/allocate prefix:
			// ip prefix or a dynamic allocation with a prefix
			if msg := validatePrefixOwner(route, r.alloc); msg != "" {
				return msg, nil
			}
			// all good prefix exists and has the proper attributes
			return "", nil
		}
		// in case of a network prefix we need to turn this in an address and validate again
		// since the initial validation is for the network subnet and not for the individual address
		// in the subnet
		if r.alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
			// if parent is prefixkind network
			if route.Labels()[ipamv1alpha1.NephioPrefixKindKey] != string(ipamv1alpha1.PrefixKindNetwork) {
				return fmt.Sprintf("an address based prefix kind can only have parent prefix kind, got: %s", route.Labels()[ipamv1alpha1.NephioPrefixKindKey]), nil
			}
			route, ok = dryrunRib.Get(r.pi.GetIPAddressPrefix())
			if !ok {
				// we can return since we know there is a parent
				// the child is a /32 or /128 which cannot have children
				return "", nil
			}
		}
		if msg := validatePrefixOwner(route, r.alloc); msg != "" {
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
		r.alloc.GetDummyLabelsFromPrefix(r.pi),
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
		if msg := r.fnc.validateChildrenExistFn(routes[0], r.alloc.GetPrefixKind()); msg != "" {
			return msg, nil
		}
	}
	// get parents
	routes = route.Parents(dryrunRib)
	if len(routes) == 0 {
		if msg := r.fnc.validateNoParentExistFn(r.alloc.GetPrefixKind(), r.alloc.GetOwnerGvk()); msg != "" {
			return msg, nil
		}
	}
	for _, route := range routes {
		if msg := r.fnc.validateParentExistFn(route, r.alloc.GetPrefixKind(), r.pi); msg != "" {
			return msg, nil
		}
	}
	return "", nil
}
