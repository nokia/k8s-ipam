package ipam

import (
	"context"
	"sync"

	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
)

//type ipamOperationFn func(c any) (IPAMOperation, error)

type IPAMOperation interface {
	Validate(ctx context.Context) (string, error)
	Apply(ctx context.Context) (*ipamv1alpha1.IPAllocation, error)
	Delete(ctx context.Context) error
}

type IPAMPrefixOperatorConfig struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	fnc   *PrefixValidatorFunctionConfig
}

type IPAMAllocOperatorConfig struct {
	alloc *ipamv1alpha1.IPAllocation
	rib   *table.RIB
	fnc   *AllocValidatorFunctionConfig
}

type IPAMOperationMapConfig struct {
	init    bool
	ipamRib ipamRib
}

type IPAMOperations interface {
	GetPrefixOperation() ipamOperation
	GetAllocOperation() ipamOperation
}

func NewIPamOperation(c *IPAMOperationMapConfig) IPAMOperations {
	return &ipamOperations{
		prefixOperation: newPrefixOperation(c),
		allocOperation:  newAllocOperation(c),
	}
}

type ipamOperations struct {
	prefixOperation ipamOperation
	allocOperation  ipamOperation
}

func (r *ipamOperations) GetPrefixOperation() ipamOperation {
	return r.prefixOperation
}

func (r *ipamOperations) GetAllocOperation() ipamOperation {
	return r.allocOperation
}

type ipamOperation interface {
	Get(alloc *ipamv1alpha1.IPAllocation) (IPAMOperation, error)
}

func newPrefixOperation(c *IPAMOperationMapConfig) ipamOperation {
	return &ipamPrefixOperation{
		init:    c.init,
		ipamRib: c.ipamRib,
		oc: map[ipamv1alpha1.PrefixKind]*PrefixValidatorFunctionConfig{
			ipamv1alpha1.PrefixKindNetwork: {
				validateExistanceOfSpecialLabelsFn: validateExistanceOfSpecialLabels,
				validateAddressPrefixFn:            validateAddressPrefix,        // no /32 or /128 allowed
				validateIfAddressinSubnetFn:        validateIfAddressinSubnetNop, // not relevant
				validateChildrenExistFn:            validateChildrenExist, 
				validateNoParentExistFn:            validateNoParentExist,
				validateParentExistFn:              validateParentExist,
			},
			ipamv1alpha1.PrefixKindLoopback: {
				validateExistanceOfSpecialLabelsFn: validateExistanceOfSpecialLabels,
				validateAddressPrefixFn:            validateAddressPrefixNop, // not relevant
				validateIfAddressinSubnetFn:        validateIfAddressinSubnet,
				validateChildrenExistFn:            validateChildrenExist,
				validateNoParentExistFn:            validateNoParentExist,
				validateParentExistFn:              validateParentExist,
			},
			ipamv1alpha1.PrefixKindPool: {
				validateExistanceOfSpecialLabelsFn: validateExistanceOfSpecialLabels,
				validateAddressPrefixFn:            validateAddressPrefix,        // no /32 or /128 allowed
				validateIfAddressinSubnetFn:        validateIfAddressinSubnetNop, // not relevant
				validateChildrenExistFn:            validateChildrenExist,
				validateNoParentExistFn:            validateNoParentExist,
				validateParentExistFn:              validateParentExist,
			},
			ipamv1alpha1.PrefixKindAggregate: {
				validateExistanceOfSpecialLabelsFn: validateExistanceOfSpecialLabels,
				validateAddressPrefixFn:            validateAddressPrefix,     // no /32 or /128 allowed
				validateIfAddressinSubnetFn:        validateIfAddressinSubnet, // not allowed
				validateChildrenExistFn:            validateChildrenExist,
				validateNoParentExistFn:            validateNoParentExist,
				validateParentExistFn:              validateParentExist,
			},
		},
	}
}

type ipamPrefixOperation struct {
	init    bool
	ipamRib ipamRib
	m       sync.Mutex
	oc      map[ipamv1alpha1.PrefixKind]*PrefixValidatorFunctionConfig
}

func (r *ipamPrefixOperation) Get(alloc *ipamv1alpha1.IPAllocation) (IPAMOperation, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// get rib, returns an error if not yet initialized based on the init flag
	rib, err := r.ipamRib.getRIB(alloc.GetNetworkInstance(), r.init)
	if err != nil {
		return nil, err
	}

	return NewPrefixOperator(&IPAMPrefixOperatorConfig{
		alloc: alloc,
		rib:   rib,
		fnc:   r.oc[alloc.GetPrefixKind()],
	})

}

func newAllocOperation(c *IPAMOperationMapConfig) ipamOperation {
	return &ipamAllocOperation{
		init:    c.init,
		ipamRib: c.ipamRib,
		oc: map[ipamv1alpha1.PrefixKind]*AllocValidatorFunctionConfig{
			ipamv1alpha1.PrefixKindNetwork: {
				validateInputFn: validateInput,
			},
			ipamv1alpha1.PrefixKindLoopback: {
				validateInputFn: validateInput,
			},
			ipamv1alpha1.PrefixKindPool: {
				validateInputFn: validateInputNop,
			},
			ipamv1alpha1.PrefixKindAggregate: {
				validateInputFn: validateInputNop,
			},
		},
	}
}

type ipamAllocOperation struct {
	init    bool
	ipamRib ipamRib
	m       sync.Mutex
	oc      map[ipamv1alpha1.PrefixKind]*AllocValidatorFunctionConfig
}

func (r *ipamAllocOperation) Get(alloc *ipamv1alpha1.IPAllocation) (IPAMOperation, error) {
	r.m.Lock()
	defer r.m.Unlock()
	// get rib, returns an error if not yet initialized based on the init flag
	rib, err := r.ipamRib.getRIB(alloc.GetNetworkInstance(), r.init)
	if err != nil {
		return nil, err
	}

	return NewAllocOperator(&IPAMAllocOperatorConfig{
		alloc: alloc,
		rib:   rib,
		fnc:   r.oc[alloc.GetPrefixKind()],
	})

}
