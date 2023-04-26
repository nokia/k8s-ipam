/*
Copyright 2023 The Nephio Authors.

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
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func pointerUint16(i uint16) *uint16 {
	return &i
}

func TestGetVLANAllocationCtx(t *testing.T) {
	cases := map[string]struct {
		v           VLANAllocation
		want        *VLANAllocationCtx
		errExpected bool
	}{
		"Dynamic": {
			v:           VLANAllocation{Spec: VLANAllocationSpec{}},
			want:        &VLANAllocationCtx{Kind: VLANAllocKindDynamic},
			errExpected: false,
		},
		"Static": {
			v:           VLANAllocation{Spec: VLANAllocationSpec{VLANID: pointerUint16(10)}},
			want:        &VLANAllocationCtx{Kind: VLANAllocKindStatic, Start: 10},
			errExpected: false,
		},
		"Range": {
			v:           VLANAllocation{Spec: VLANAllocationSpec{VLANRange: pointer.String("100:199")}},
			want:        &VLANAllocationCtx{Kind: VLANAllocKindRange, Start: 100, Size: 100},
			errExpected: false,
		},
		"Size": {
			v:           VLANAllocation{Spec: VLANAllocationSpec{VLANRange: pointer.String("100")}},
			want:        &VLANAllocationCtx{Kind: VLANAllocKindSize, Size: 100},
			errExpected: false,
		},
		"Error": {
			v:           VLANAllocation{Spec: VLANAllocationSpec{VLANRange: pointer.String("1:2:3")}},
			want:        nil,
			errExpected: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got, err := tc.v.GetVLANAllocationCtx()
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

func TestAddOwnerLabelsToCR(t *testing.T) {
	cases := map[string]struct {
		o    *VLANAllocation
		want map[string]string
	}{
		"NoLabels": {
			o: &VLANAllocation{
				TypeMeta: metav1.TypeMeta{
					Kind:       VLANAllocationKind,
					APIVersion: GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
				},
				Spec: VLANAllocationSpec{},
			},
			want: map[string]string{
				allocv1alpha1.NephioGvkKey:               "VLANAllocation.v1alpha1.vlan.alloc.nephio.org",
				allocv1alpha1.NephioNsnNameKey:           "a",
				allocv1alpha1.NephioNsnNamespaceKey:      "default",
				allocv1alpha1.NephioOwnerGvkKey:          "VLANAllocation.v1alpha1.vlan.alloc.nephio.org",
				allocv1alpha1.NephioOwnerNsnNameKey:      "a",
				allocv1alpha1.NephioOwnerNsnNamespaceKey: "default",
			},
		},
		"Labels": {
			o: &VLANAllocation{
				TypeMeta: metav1.TypeMeta{
					Kind:       VLANAllocationKind,
					APIVersion: GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
					Labels: map[string]string{
						allocv1alpha1.NephioOwnerGvkKey:          "A.v1alpha1.nephio.org",
						allocv1alpha1.NephioOwnerNsnNameKey:      "y",
						allocv1alpha1.NephioOwnerNsnNamespaceKey: "x",
					},
				},

				Spec: VLANAllocationSpec{
					AllocationLabels: allocv1alpha1.AllocationLabels{
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								"a": "b",
								"c": "d",
							},
						},
					},
				},
			},
			want: map[string]string{
				allocv1alpha1.NephioGvkKey:               "VLANAllocation.v1alpha1.vlan.alloc.nephio.org",
				allocv1alpha1.NephioNsnNameKey:           "a",
				allocv1alpha1.NephioNsnNamespaceKey:      "default",
				allocv1alpha1.NephioOwnerGvkKey:          "A.v1alpha1.nephio.org",
				allocv1alpha1.NephioOwnerNsnNameKey:      "y",
				allocv1alpha1.NephioOwnerNsnNamespaceKey: "x",
				"a":                                      "b",
				"c":                                      "d",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			tc.o.AddOwnerLabelsToCR()
			got := tc.o.GetUserDefinedLabels()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}

		})
	}
}
