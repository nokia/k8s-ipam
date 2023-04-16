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
)

func TestIPAllocationCondition(t *testing.T) {

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
			x := &IPAllocation{}
			// set conditions
			x.SetConditions(tc.cs...)

			got := x.GetCondition(tc.t)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestIPPrefixCondition: -want, +got:\n%s", diff)
			}
		})
	}
}
