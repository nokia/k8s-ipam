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
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func pointerUint16(i uint16) *uint16 {
	return &i
}

func TestGetVLANClaimCtx(t *testing.T) {
	cases := map[string]struct {
		v           VLANClaim
		want        *VLANClaimCtx
		errExpected bool
	}{
		"Dynamic": {
			v:           VLANClaim{Spec: VLANClaimSpec{}},
			want:        &VLANClaimCtx{Kind: VLANClaimTypeDynamic},
			errExpected: false,
		},
		"Static": {
			v:           VLANClaim{Spec: VLANClaimSpec{VLANID: pointerUint16(10)}},
			want:        &VLANClaimCtx{Kind: VLANClaimTypeStatic, Start: 10},
			errExpected: false,
		},
		"Range": {
			v:           VLANClaim{Spec: VLANClaimSpec{VLANRange: pointer.String("100:199")}},
			want:        &VLANClaimCtx{Kind: VLANClaimTypeRange, Start: 100, Size: 100},
			errExpected: false,
		},
		"Size": {
			v:           VLANClaim{Spec: VLANClaimSpec{VLANRange: pointer.String("100")}},
			want:        &VLANClaimCtx{Kind: VLANClaimTypeSize, Size: 100},
			errExpected: false,
		},
		"Error": {
			v:           VLANClaim{Spec: VLANClaimSpec{VLANRange: pointer.String("1:2:3")}},
			want:        nil,
			errExpected: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got, err := tc.v.GetVLANClaimCtx()
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
		o    *VLANClaim
		want map[string]string
	}{
		"NoLabels": {
			o: &VLANClaim{
				TypeMeta: metav1.TypeMeta{
					Kind:       VLANClaimKind,
					APIVersion: GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
				},
				Spec: VLANClaimSpec{},
			},
			want: map[string]string{
				resourcev1alpha1.NephioGvkKey:               "VLANClaim.v1alpha1.vlan.resource.nephio.org",
				resourcev1alpha1.NephioNsnNameKey:           "a",
				resourcev1alpha1.NephioNsnNamespaceKey:      "default",
				resourcev1alpha1.NephioOwnerGvkKey:          "VLANClaim.v1alpha1.vlan.resource.nephio.org",
				resourcev1alpha1.NephioOwnerNsnNameKey:      "a",
				resourcev1alpha1.NephioOwnerNsnNamespaceKey: "default",
			},
		},
		"Labels": {
			o: &VLANClaim{
				TypeMeta: metav1.TypeMeta{
					Kind:       VLANClaimKind,
					APIVersion: GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
					Labels: map[string]string{
						resourcev1alpha1.NephioOwnerGvkKey:          "A.v1alpha1.nephio.org",
						resourcev1alpha1.NephioOwnerNsnNameKey:      "y",
						resourcev1alpha1.NephioOwnerNsnNamespaceKey: "x",
					},
				},

				Spec: VLANClaimSpec{
					ClaimLabels: resourcev1alpha1.ClaimLabels{
						UserDefinedLabels: resourcev1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								"a": "b",
								"c": "d",
							},
						},
					},
				},
			},
			want: map[string]string{
				resourcev1alpha1.NephioGvkKey:               "VLANClaim.v1alpha1.vlan.resource.nephio.org",
				resourcev1alpha1.NephioNsnNameKey:           "a",
				resourcev1alpha1.NephioNsnNamespaceKey:      "default",
				resourcev1alpha1.NephioOwnerGvkKey:          "A.v1alpha1.nephio.org",
				resourcev1alpha1.NephioOwnerNsnNameKey:      "y",
				resourcev1alpha1.NephioOwnerNsnNamespaceKey: "x",
				"a": "b",
				"c": "d",
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
