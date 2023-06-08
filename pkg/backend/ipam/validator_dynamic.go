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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DynamicValidatorFunctionConfig struct {
	validateInputFn validateInputFn
}

type ClaimValidatorConfig struct {
	claim *ipamv1alpha1.IPClaim
	rib   *table.RIB
	fnc   *DynamicValidatorFunctionConfig
}

func NewClaimValidator(c *ClaimValidatorConfig) Validator {
	return &claimvalidator{
		claim: c.claim,
		rib:   c.rib,
		fnc:   c.fnc,
	}
}

type claimvalidator struct {
	claim *ipamv1alpha1.IPClaim
	rib   *table.RIB
	fnc   *DynamicValidatorFunctionConfig
	l     logr.Logger
}

func (r *claimvalidator) Validate(ctx context.Context) (string, error) {
	r.l = log.FromContext(ctx).WithValues("prefixkind", r.claim.Spec.Kind, "cr", r.claim.GetGenericNamespacedName())
	r.l.Info("validate claim without prefix")

	// validate input
	if msg := r.fnc.validateInputFn(r.claim, nil); msg != "" {
		return msg, nil
	}

	return "", nil
}
