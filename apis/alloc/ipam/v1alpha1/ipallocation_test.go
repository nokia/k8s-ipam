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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestIsCreatePrefixAllcationValid(t *testing.T) {
	tests := map[string]struct {
		o           *IPAllocation
		want        bool
		errExpected bool
	}{
		"CreatePrefixWithPrefix": {
			o: &IPAllocation{
				Spec: IPAllocationSpec{
					Kind:         PrefixKindNetwork,
					CreatePrefix: pointer.Bool(true),
					Prefix:       pointer.String("10.0.0.0/24"),
				},
			},
			want:        true,
			errExpected: false,
		},
		"CreatePrefixWithPrefixLength": {
			o: &IPAllocation{
				Spec: IPAllocationSpec{
					Kind:         PrefixKindNetwork,
					CreatePrefix: pointer.Bool(true),
					PrefixLength: util.PointerUint8(24),
				},
			},
			want:        true,
			errExpected: false,
		},
		"CreatePrefixEmpty": {
			o: &IPAllocation{
				Spec: IPAllocationSpec{
					Kind:         PrefixKindNetwork,
					CreatePrefix: pointer.Bool(true),
				},
			},
			want:        false,
			errExpected: true,
		},
		"CreatePrefixWithAddressPrefix": {
			o: &IPAllocation{
				Spec: IPAllocationSpec{
					Kind:         PrefixKindNetwork,
					CreatePrefix: pointer.Bool(true),
					Prefix:       pointer.String("10.0.0.4/32"),
				},
			},
			want:        false,
			errExpected: true,
		},
		"CreateAddress": {
			o: &IPAllocation{
				Spec: IPAllocationSpec{
					Kind:   PrefixKindNetwork,
					Prefix: pointer.String("10.0.0.4/32"),
				},
			},
			want:        false,
			errExpected: false,
		},
		"CreatePrefixWithoutCreatePrefix": {
			o: &IPAllocation{
				Spec: IPAllocationSpec{
					Kind:   PrefixKindNetwork,
					Prefix: pointer.String("10.0.0.4/24"),
				},
			},
			want:        false,
			errExpected: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tc.o.IsCreatePrefixAllcationValid()
			if tc.errExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Errorf("-want, +got:\n%s", diff)
				}
			}
		})
	}
}
