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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNICondition(t *testing.T) {

	conditionSynced := allocv1alpha1.Condition{Kind: allocv1alpha1.ConditionKindSynced, Status: corev1.ConditionTrue, Message: "synced"}
	conditionReady := allocv1alpha1.Condition{Kind: allocv1alpha1.ConditionKindReady, Status: corev1.ConditionTrue, Message: "ready"}

	cases := map[string]struct {
		cs   []allocv1alpha1.Condition
		t    allocv1alpha1.ConditionKind
		want allocv1alpha1.Condition
	}{
		"ConditionExists": {
			cs:   []allocv1alpha1.Condition{conditionSynced, conditionReady},
			t:    allocv1alpha1.ConditionKindSynced,
			want: allocv1alpha1.Condition{Kind: allocv1alpha1.ConditionKindSynced, Status: corev1.ConditionTrue, Message: "synced"},
		},
		"ConditionDoesNotExist": {
			cs:   []allocv1alpha1.Condition{conditionSynced, conditionReady},
			t:    "dummy",
			want: allocv1alpha1.Condition{Kind: "dummy", Status: corev1.ConditionUnknown},
		},
		"ConditionEmpty": {
			cs:   []allocv1alpha1.Condition{},
			t:    allocv1alpha1.ConditionKindSynced,
			want: allocv1alpha1.Condition{Kind: allocv1alpha1.ConditionKindSynced, Status: corev1.ConditionUnknown},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// initialize ni
			ni := &NetworkInstance{}
			// set conditions
			ni.SetConditions(tc.cs...)

			got := ni.GetCondition(tc.t)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestNICondition: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNiGetPrefixes(t *testing.T) {
	tests := map[string]struct {
		input *NetworkInstance
		want  []*Prefix
	}{
		"GetPrefixes Empty": {
			input: &NetworkInstance{
				Spec: NetworkInstanceSpec{},
			},
			want: []*Prefix{},
		},
		"GetPrefixes Single": {
			input: &NetworkInstance{
				Spec: NetworkInstanceSpec{
					Prefixes: []*Prefix{
						{
							Prefix: "1000::/12",
							Labels: map[string]string{"a": "a"},
						},
					},
				},
			},
			want: []*Prefix{
				{
					Prefix: "1000::/12",
					Labels: map[string]string{"a": "a"},
				},
			},
		},
		"GetPrefixes Multiple": {
			input: &NetworkInstance{
				Spec: NetworkInstanceSpec{
					Prefixes: []*Prefix{
						{
							Prefix: "1000::/12",
							Labels: map[string]string{"a": "a"},
						},
						{
							Prefix: "10.0.0.0/8",
							Labels: map[string]string{"b": "b"},
						},
					},
				},
			},
			want: []*Prefix{
				{
					Prefix: "1000::/12",
					Labels: map[string]string{"a": "a"},
				},
				{
					Prefix: "10.0.0.0/8",
					Labels: map[string]string{"b": "b"},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotPrefixes := tc.input.GetPrefixes()

			if len(gotPrefixes) != len(tc.want) {
				t.Errorf("TestNiGetPrefixes: unexpected length -want %d, +got: %d\n", len(tc.want), len(gotPrefixes))
			} else {
				for i, got := range gotPrefixes {
					if diff := cmp.Diff(tc.want[i].GetPrefix(), got.GetPrefix()); diff != "" {
						t.Errorf("TestNiGetPrefixes prefix: -want, +got:\n%s", diff)
					}
					if diff := cmp.Diff(tc.want[i].GetPrefixLabels(), got.GetPrefixLabels()); diff != "" {
						t.Errorf("TestNiGetPrefixes labels: -want, +got:\n%s", diff)
					}
				}
			}
		})
	}
}

func TestNiGetNamespacedName(t *testing.T) {
	tests := map[string]struct {
		input       *NetworkInstance
		inputPrefix string
		want        string
	}{
		"GetNamespacedName": {
			input: &NetworkInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "b",
				},
			},
			want: "b/a",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.input.GetNamespacedName().String()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestNiGetNamespacedName: -want, +got:\n%s", diff)
			}

		})
	}
}

func TestNiGetNameFromNetworkInstancePrefix(t *testing.T) {
	tests := map[string]struct {
		input       *NetworkInstance
		inputPrefix string
		want        string
	}{
		"GetNameFromNetworkInstancePrefix": {
			input: &NetworkInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "b",
				},
			},
			inputPrefix: "10.0.0.1/24",
			want:        "a-aggregate-10.0.0.1-24",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.input.GetNameFromNetworkInstancePrefix(tc.inputPrefix)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestNiGetNameFromNetworkInstancePrefix: -want, +got:\n%s", diff)
			}

		})
	}
}

func TestNiGetNetworkInstanceNameSpace(t *testing.T) {
	tests := map[string]struct {
		input *NetworkInstance
		want  string
	}{
		"GetNetworkInstanceNameSpace": {
			input: &NetworkInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "b",
				},
			},
			want: "b-a",
		},
		"GetNetworkInstanceNameSpaceEmptyNamespace": {
			input: &NetworkInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
				},
			},
			want: "a",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.input.GetGenericNamespacedName()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestNiGetNetworkInstanceNameSpace: -want, +got:\n%s", diff)
			}

		})
	}
}

func TestNiGetAllocatedPrefixes(t *testing.T) {
	tests := map[string]struct {
		input *NetworkInstance
		want  []*Prefix
	}{
		"GetAllocatedPrefixes Empty": {
			input: &NetworkInstance{
				Status: NetworkInstanceStatus{},
			},
			want: []*Prefix{},
		},
		"GetAllocatedPrefixes Single": {
			input: &NetworkInstance{
				Status: NetworkInstanceStatus{
					AllocatedPrefixes: []*Prefix{
						{
							Prefix: "1000::/12",
							Labels: map[string]string{"a": "a"},
						},
					},
				},
			},
			want: []*Prefix{
				{
					Prefix: "1000::/12",
					Labels: map[string]string{"a": "a"},
				},
			},
		},
		"GetAllocatedPrefixes Multiple": {
			input: &NetworkInstance{
				Status: NetworkInstanceStatus{
					AllocatedPrefixes: []*Prefix{
						{
							Prefix: "1000::/12",
							Labels: map[string]string{"a": "a"},
						},
						{
							Prefix: "10.0.0.0/8",
							Labels: map[string]string{"b": "b"},
						},
					},
				},
			},
			want: []*Prefix{
				{
					Prefix: "1000::/12",
					Labels: map[string]string{"a": "a"},
				},
				{
					Prefix: "10.0.0.0/8",
					Labels: map[string]string{"b": "b"},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotPrefixes := tc.input.GetAllocatedPrefixes()

			if len(gotPrefixes) != len(tc.want) {
				t.Errorf("TestNiGetAllocatedPrefixes: unexpected length -want %d, +got: %d\n", len(tc.want), len(gotPrefixes))
			} else {
				for i, got := range gotPrefixes {
					if diff := cmp.Diff(tc.want[i].GetPrefix(), got.GetPrefix()); diff != "" {
						t.Errorf("TestNiGetAllocatedPrefixes prefix: -want, +got:\n%s", diff)
					}
					if diff := cmp.Diff(tc.want[i].GetPrefixLabels(), got.GetPrefixLabels()); diff != "" {
						t.Errorf("TestNiGetAllocatedPrefixes labels: -want, +got:\n%s", diff)
					}
				}
			}
		})
	}
}
