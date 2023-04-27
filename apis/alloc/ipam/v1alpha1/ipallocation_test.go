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
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/util"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestBuildIPAllocationFromNetworkInstancePrefix(t *testing.T) {
	tests := map[string]struct {
		o    *NetworkInstance
		p    Prefix
		want *IPAllocation
	}{
		"ConditionExists": {
			o: &NetworkInstance{
				TypeMeta: metav1.TypeMeta{
					Kind:       NetworkInstanceKind,
					APIVersion: SchemeBuilder.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
				},
				Spec: NetworkInstanceSpec{},
			},
			p: Prefix{
				Prefix: "10.0.0.0/8",
				UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
					Labels: map[string]string{
						"x": "y",
						"v": "w",
					},
				},
			},
			want: &IPAllocation{
				TypeMeta: metav1.TypeMeta{
					Kind:       IPAllocationKind,
					APIVersion: SchemeBuilder.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a-aggregate-10.0.0.0-8",
					Namespace: "default",
				},
				Spec: IPAllocationSpec{
					Kind: PrefixKindAggregate,
					NetworkInstance: corev1.ObjectReference{
						Name:      "a",
						Namespace: "default",
					},
					Prefix:       pointer.String("10.0.0.0/8"),
					PrefixLength: util.PointerUint8(8),
					CreatePrefix: pointer.Bool(true),
					AllocationLabels: allocv1alpha1.AllocationLabels{
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioGvkKey:               "IPAllocation.v1alpha1.ipam.alloc.nephio.org",
								allocv1alpha1.NephioNsnNameKey:           "a-aggregate-10.0.0.0-8",
								allocv1alpha1.NephioNsnNamespaceKey:      "default",
								allocv1alpha1.NephioOwnerGvkKey:          "NetworkInstance.v1alpha1.ipam.alloc.nephio.org",
								allocv1alpha1.NephioOwnerNsnNameKey:      "a",
								allocv1alpha1.NephioOwnerNsnNamespaceKey: "default",
								"v":                                      "w",
								"x":                                      "y",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := BuildIPAllocationFromNetworkInstancePrefix(tc.o, tc.p)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestBuildIPAllocationFromIPPrefix(t *testing.T) {
	tests := map[string]struct {
		o    *IPPrefix
		want *IPAllocation
	}{
		"ConditionExists": {
			o: &IPPrefix{
				TypeMeta: metav1.TypeMeta{
					Kind:       IPPrefixKind,
					APIVersion: SchemeBuilder.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
				},
				Spec: IPPrefixSpec{
					Kind: PrefixKindNetwork,
					NetworkInstance: corev1.ObjectReference{
						Name: "vpc",
					},
					Prefix: "10.0.0.3/24",
					UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
						Labels: map[string]string{
							"x": "y",
							"v": "w",
						},
					},
				},
			},
			want: &IPAllocation{
				TypeMeta: metav1.TypeMeta{
					Kind:       IPAllocationKind,
					APIVersion: SchemeBuilder.GroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
				},
				Spec: IPAllocationSpec{
					Kind: PrefixKindNetwork,
					NetworkInstance: corev1.ObjectReference{
						Name: "vpc",
					},
					Prefix:       pointer.String("10.0.0.3/24"),
					PrefixLength: util.PointerUint8(24),
					CreatePrefix: pointer.Bool(true),
					AllocationLabels: allocv1alpha1.AllocationLabels{
						UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
							Labels: map[string]string{
								allocv1alpha1.NephioGvkKey:               "IPAllocation.v1alpha1.ipam.alloc.nephio.org",
								allocv1alpha1.NephioNsnNameKey:           "a",
								allocv1alpha1.NephioNsnNamespaceKey:      "default",
								allocv1alpha1.NephioOwnerGvkKey:          "IPPrefix.v1alpha1.ipam.alloc.nephio.org",
								allocv1alpha1.NephioOwnerNsnNameKey:      "a",
								allocv1alpha1.NephioOwnerNsnNamespaceKey: "default",
								"v":                                      "w",
								"x":                                      "y",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := BuildIPAllocationFromIPPrefix(tc.o)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

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
