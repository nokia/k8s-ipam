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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestVLANGetCondition(t *testing.T) {
	tests := map[string]struct {
		cs   *resourcev1alpha1.ConditionedStatus
		t    resourcev1alpha1.ConditionType
		want resourcev1alpha1.Condition
	}{
		"ConditionExists": {
			cs:   resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.Ready()),
			t:    resourcev1alpha1.ConditionTypeReady,
			want: resourcev1alpha1.Ready(),
		},
		"ConditionDoesNotExist": {
			cs: resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.Ready()),
			t:  resourcev1alpha1.ConditionTypeSynced,
			want: resourcev1alpha1.Condition{Condition: metav1.Condition{
				Type:   string(resourcev1alpha1.ConditionTypeSynced),
				Status: metav1.ConditionFalse,
			}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			o := VLAN{}
			o.Status.ConditionedStatus = *tc.cs
			got := o.GetCondition(resourcev1alpha1.ConditionType(tc.t))

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestVLANSetConditions(t *testing.T) {
	cases := map[string]struct {
		cs   *resourcev1alpha1.ConditionedStatus
		c    []resourcev1alpha1.Condition
		want *resourcev1alpha1.ConditionedStatus
	}{
		"TypeIsIdentical": {
			cs:   resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.Ready()),
			c:    []resourcev1alpha1.Condition{resourcev1alpha1.Ready()},
			want: resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.Ready()),
		},

		"TypeUnEqual": {
			cs:   resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.Unknown()),
			c:    []resourcev1alpha1.Condition{resourcev1alpha1.Ready()},
			want: resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.Ready()),
		},
		"TypeDoesNotExist": {
			cs:   resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.ReconcileSuccess()),
			c:    []resourcev1alpha1.Condition{resourcev1alpha1.Ready()},
			want: resourcev1alpha1.NewConditionedStatus(resourcev1alpha1.ReconcileSuccess(), resourcev1alpha1.Ready()),
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
