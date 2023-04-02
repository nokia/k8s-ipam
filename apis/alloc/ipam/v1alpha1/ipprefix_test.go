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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestIPPrefixCondition(t *testing.T) {

	conditionSynced := allocv1alpha1.Condition{Kind: allocv1alpha1.ConditionKindSynced, Status: v1.ConditionTrue, Message: "synced"}
	conditionReady := allocv1alpha1.Condition{Kind: allocv1alpha1.ConditionKindReady, Status: v1.ConditionTrue, Message: "ready"}

	cases := map[string]struct {
		cs   []allocv1alpha1.Condition
		t    allocv1alpha1.ConditionKind
		want allocv1alpha1.Condition
	}{
		"ConditionExists": {
			cs:   []allocv1alpha1.Condition{conditionSynced, conditionReady},
			t:    allocv1alpha1.ConditionKindSynced,
			want: allocv1alpha1.Condition{Kind: allocv1alpha1.ConditionKindSynced, Status: v1.ConditionTrue, Message: "synced"},
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
			x := &IPPrefix{}
			// set conditions
			x.SetConditions(tc.cs...)

			got := x.GetCondition(tc.t)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestIPPrefixCondition: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetIPPrefixNameSpace(t *testing.T) {
	tests := map[string]struct {
		input *IPPrefix
		want  string
	}{
		"TestGetIPPrefixNameSpace": {
			input: &IPPrefix{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "b",
				},
			},
			want: "b-a",
		},
		"GetNetworkInstanceNameSpaceEmptyNamespace": {
			input: &IPPrefix{
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
				t.Errorf("TestGetIPPrefixNameSpace: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIPPrefixGetNetworkInstance(t *testing.T) {
	tests := map[string]struct {
		input *IPPrefix
		want  types.NamespacedName
	}{
		"GetNetworkInstanceEmpty": {
			input: &IPPrefix{
				Spec: IPPrefixSpec{},
			},
			want: types.NamespacedName{},
		},
		"GetNetworkInstanceName": {
			input: &IPPrefix{
				Spec: IPPrefixSpec{
					NetworkInstance: &corev1.ObjectReference{
						Name: "a",
					},
				},
			},
			want: types.NamespacedName{
				Name: "a",
			},
		},
		"GetNetworkInstanceNameSpace": {
			input: &IPPrefix{
				Spec: IPPrefixSpec{
					NetworkInstance: &corev1.ObjectReference{
						Namespace: "a",
					},
				},
			},
			want: types.NamespacedName{
				Namespace: "a",
			},
		},
		"GetNetworkInstanceNameSpaceName": {
			input: &IPPrefix{
				Spec: IPPrefixSpec{
					NetworkInstance: &corev1.ObjectReference{
						Namespace: "a",
						Name:      "a",
					},
				},
			},
			want: types.NamespacedName{
				Namespace: "a",
				Name:      "a",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.input.GetNetworkInstance()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestIPPrefixGetNetworkInstance: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIPPrefixGetPrefix(t *testing.T) {
	tests := map[string]struct {
		input *IPPrefix
		want  string
	}{
		"GetPrefix": {
			input: &IPPrefix{
				Spec: IPPrefixSpec{
					Prefix: "10.0.0.1/24",
				},
			},
			want: "10.0.0.1/24",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.input.GetPrefix()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestIPPrefixGetPrefix: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIPPrefixGetSpecLabels(t *testing.T) {
	tests := map[string]struct {
		input *IPPrefix
		want  any
	}{
		"GetSpecLabels Empty": {
			input: &IPPrefix{
				Spec: IPPrefixSpec{
					Prefix: "10.0.0.1/24",
				},
			},
			want: map[string]string{},
		},
		"GetSpecLabels": {
			input: &IPPrefix{
				Spec: IPPrefixSpec{
					Prefix: "10.0.0.1/24",
					Labels: map[string]string{"a": "b"},
				},
			},
			want: map[string]string{"a": "b"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.input.GetSpecLabels()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestIPPrefixGetSpecLabels: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIPPrefixGetAllocatedPrefix(t *testing.T) {
	tests := map[string]struct {
		input *IPPrefix
		want  string
	}{
		"GetPrefix": {
			input: &IPPrefix{
				Status: IPPrefixStatus{
					AllocatedPrefix: "10.0.0.1/24",
				},
			},
			want: "10.0.0.1/24",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.input.GetAllocatedPrefix()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestIPPrefixGetAllocatedPrefix: -want, +got:\n%s", diff)
			}
		})
	}
}
