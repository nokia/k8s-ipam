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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionEqual(t *testing.T) {
	cases := map[string]struct {
		a    Condition
		b    Condition
		want bool
	}{
		"IdenticalIgnoringTimestamp": {
			a:    Condition{metav1.Condition{Type: string(ConditionTypeReady), LastTransitionTime: metav1.Now()}},
			b:    Condition{metav1.Condition{Type: string(ConditionTypeReady), LastTransitionTime: metav1.Now()}},
			want: true,
		},
		"DifferentType": {
			a:    Condition{metav1.Condition{Type: string(ConditionTypeReady)}},
			b:    Condition{metav1.Condition{Type: string(ConditionTypeSynced)}},
			want: false,
		},
		"DifferentStatus": {
			a:    Condition{metav1.Condition{Status: metav1.ConditionTrue}},
			b:    Condition{metav1.Condition{Status: metav1.ConditionFalse}},
			want: false,
		},
		"DifferentReason": {
			a:    Condition{metav1.Condition{Reason: Ready().Reason}},
			b:    Condition{metav1.Condition{Reason: ReconcileSuccess().Reason}},
			want: false,
		},
		"DifferentMessage": {
			a:    Condition{metav1.Condition{Message: "a"}},
			b:    Condition{metav1.Condition{Message: "b"}},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.a.Equal(tc.b)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestConditionedStatusEqual(t *testing.T) {
	cases := map[string]struct {
		a    *ConditionedStatus
		b    *ConditionedStatus
		want bool
	}{
		"Identical": {
			a:    NewConditionedStatus(Ready(), ReconcileSuccess()),
			b:    NewConditionedStatus(Ready(), ReconcileSuccess()),
			want: true,
		},
		"IdenticalDifferentOrder": {
			a:    NewConditionedStatus(Unknown(), ReconcileSuccess()),
			b:    NewConditionedStatus(ReconcileSuccess(), Unknown()),
			want: true,
		},
		"DifferentAmount": {
			a:    NewConditionedStatus(Unknown(), ReconcileSuccess()),
			b:    NewConditionedStatus(ReconcileSuccess()),
			want: false,
		},
		"DifferentCondition": {
			a:    NewConditionedStatus(Ready(), ReconcileSuccess()),
			b:    NewConditionedStatus(Ready(), ReconcileError(errors.New("a"))),
			want: false,
		},
		"Anil": {
			a:    nil,
			b:    NewConditionedStatus(Ready(), ReconcileSuccess()),
			want: false,
		},
		"Bnil": {
			a:    NewConditionedStatus(Ready(), ReconcileSuccess()),
			b:    nil,
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.a.Equal(tc.b)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestSetConditions(t *testing.T) {
	cases := map[string]struct {
		cs   *ConditionedStatus
		c    []Condition
		want *ConditionedStatus
	}{
		"TypeIsIdentical": {
			cs:   NewConditionedStatus(Ready()),
			c:    []Condition{Ready()},
			want: NewConditionedStatus(Ready()),
		},

		"TypeUnEqual": {
			cs:   NewConditionedStatus(Unknown()),
			c:    []Condition{Ready()},
			want: NewConditionedStatus(Ready()),
		},
		"TypeDoesNotExist": {
			cs:   NewConditionedStatus(ReconcileSuccess()),
			c:    []Condition{Ready()},
			want: NewConditionedStatus(ReconcileSuccess(), Ready()),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.SetConditions(tc.c...)

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetCondition(t *testing.T) {
	cases := map[string]struct {
		cs   *ConditionedStatus
		t    ConditionType
		want Condition
	}{
		"ConditionExists": {
			cs:   NewConditionedStatus(Ready()),
			t:    ConditionTypeReady,
			want: Ready(),
		},
		"ConditionDoesNotExist": {
			cs: NewConditionedStatus(Ready()),
			t:  ConditionTypeSynced,
			want: Condition{metav1.Condition{
				Type:   string(ConditionTypeSynced),
				Status: metav1.ConditionFalse,
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.cs.GetCondition(tc.t)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestConditionWithMessage(t *testing.T) {
	testMsg := "abcdefg"
	cases := map[string]struct {
		c    Condition
		msg  string
		want Condition
	}{
		"MessageAdded": {
			c:    Condition{metav1.Condition{Type: string(ConditionTypeReady), Reason: string(ConditionReasonReady)}},
			msg:  testMsg,
			want: Condition{metav1.Condition{Type: string(ConditionTypeReady), Reason: string(ConditionReasonReady), Message: testMsg}},
		},
		"MessageChanged": {
			c:    Condition{metav1.Condition{Type: string(ConditionTypeReady), Reason: string(ConditionReasonReady), Message: "aaaaaaaaa"}},
			msg:  testMsg,
			want: Condition{metav1.Condition{Type: string(ConditionTypeReady), Reason: string(ConditionReasonReady), Message: testMsg}},
		},
		"MessageCleared": {
			c:    Condition{metav1.Condition{Type: string(ConditionTypeReady), Reason: string(ConditionReasonReady), Message: testMsg}},
			msg:  "",
			want: Condition{metav1.Condition{Type: string(ConditionTypeReady), Reason: string(ConditionReasonReady)}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.c.WithMessage(tc.msg)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}
