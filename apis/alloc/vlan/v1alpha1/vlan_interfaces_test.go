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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestVLANGetCondition(t *testing.T) {
	tests := map[string]struct {
		cs   *allocv1alpha1.ConditionedStatus
		t    allocv1alpha1.ConditionType
		want allocv1alpha1.Condition
	}{
		"ConditionExists": {
			cs:   allocv1alpha1.NewConditionedStatus(allocv1alpha1.Ready()),
			t:    allocv1alpha1.ConditionTypeReady,
			want: allocv1alpha1.Ready(),
		},
		"ConditionDoesNotExist": {
			cs: allocv1alpha1.NewConditionedStatus(allocv1alpha1.Ready()),
			t:  allocv1alpha1.ConditionTypeSynced,
			want: allocv1alpha1.Condition{Condition: metav1.Condition{
				Type:   string(allocv1alpha1.ConditionTypeSynced),
				Status: metav1.ConditionFalse,
			}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			o := VLAN{}
			o.Status.ConditionedStatus = *tc.cs
			got := o.GetCondition(allocv1alpha1.ConditionType(tc.t))

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestVLANSetConditions(t *testing.T) {
	cases := map[string]struct {
		cs   *allocv1alpha1.ConditionedStatus
		c    []allocv1alpha1.Condition
		want *allocv1alpha1.ConditionedStatus
	}{
		"TypeIsIdentical": {
			cs:   allocv1alpha1.NewConditionedStatus(allocv1alpha1.Ready()),
			c:    []allocv1alpha1.Condition{allocv1alpha1.Ready()},
			want: allocv1alpha1.NewConditionedStatus(allocv1alpha1.Ready()),
		},

		"TypeUnEqual": {
			cs:   allocv1alpha1.NewConditionedStatus(allocv1alpha1.Unknown()),
			c:    []allocv1alpha1.Condition{allocv1alpha1.Ready()},
			want: allocv1alpha1.NewConditionedStatus(allocv1alpha1.Ready()),
		},
		"TypeDoesNotExist": {
			cs:   allocv1alpha1.NewConditionedStatus(allocv1alpha1.ReconcileSuccess()),
			c:    []allocv1alpha1.Condition{allocv1alpha1.Ready()},
			want: allocv1alpha1.NewConditionedStatus(allocv1alpha1.ReconcileSuccess(), allocv1alpha1.Ready()),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := VLAN{}
			o.Status.ConditionedStatus = *tc.cs
			o.SetConditions(tc.c...)

			got := &o.Status.ConditionedStatus
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestVLANGetGenericNamespacedName(t *testing.T) {
	cases := map[string]struct {
		nsn  types.NamespacedName
		want string
	}{
		"NamespaceName": {
			nsn:  types.NamespacedName{Namespace: "a", Name: "b"},
			want: "a-b",
		},

		"NameOnly": {
			nsn:  types.NamespacedName{Name: "b"},
			want: "b",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := VLAN{}
			o.SetName(tc.nsn.Name)
			o.SetNamespace(tc.nsn.Namespace)
			got := o.GetGenericNamespacedName()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestVLANGetUserDefinedLabels(t *testing.T) {
	cases := map[string]struct {
		labels map[string]string
		want   map[string]string
	}{
		"Labels": {
			labels: map[string]string{"a": "b", "c": "d"},
			want:   map[string]string{"a": "b", "c": "d"},
		},

		"Nil": {
			labels: nil,
			want:   map[string]string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := VLAN{}
			v.Spec.Labels = tc.labels

			got := v.GetUserDefinedLabels()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("TestGetSpecLabels(...): -want, +got:\n%s", diff)
			}
		})
	}
}
