package ipam

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type validateInputFn func(alloc *ipamv1alpha1.IPAllocation) string

func validateInputNop(alloc *ipamv1alpha1.IPAllocation) string { return "" }

type AllocValidatorFunctionConfig struct {
	validateInputFn validateInputFn
}

type AllocValidatorConfig struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	fnc   *AllocValidatorFunctionConfig
}

func NewAllocValidator(c *AllocValidatorConfig) Validator {
	return &allocvalidator{
		alloc: c.alloc,
		rib:   c.rib,
		fnc:   c.fnc,
	}
}

type allocvalidator struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	fnc   *AllocValidatorFunctionConfig
	l     logr.Logger
}

func (r *allocvalidator) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("prefixkind", r.alloc.GetPrefixKind(), "cr", r.alloc.GetGenericNamespacedName())
	r.l.Info("validate alloc without prefix")

	// validate input
	fmt.Printf("validate fnc: %v\n", r.fnc)
	r.l.Info("validate", "fnc", r.fnc)
	if msg := r.fnc.validateInputFn(r.alloc); msg != "" {
		return msg, nil
	}

	return "", nil
}

func validateInput(alloc *ipamv1alpha1.IPAllocation) string {
	// we expect some label metadata in the spec for prefixes that
	// are either statically provisioned (we dont expect /32 or /128 in a network prefix)
	// for dynamically allocated prefixes we expecte labels, exception is interface specific allocations(which have prefixlength undefined)
	if alloc.GetPrefixLengthFromSpec().Int() != 0 && len(alloc.GetSpecLabels()) == 0 {
		return "prefix cannot have empty labels, otherwise specific selection is not possible"
	}
	return ""
}
