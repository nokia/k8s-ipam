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
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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
	r.l = log.FromContext(ctx).WithValues("prefixkind", r.alloc.Spec.Kind, "cr", r.alloc.GetGenericNamespacedName())
	r.l.Info("validate alloc without prefix")

	// validate input
	if msg := r.fnc.validateInputFn(r.alloc, nil); msg != "" {
		return msg, nil
	}

	return "", nil
}
