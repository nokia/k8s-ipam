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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetUserDefinedLabels(t *testing.T) {
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
			o := UserDefinedLabels{
				Labels: tc.labels,
			}

			got := o.GetUserDefinedLabels()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetSelectorLabels(t *testing.T) {
	cases := map[string]struct {
		selector *metav1.LabelSelector
		want     map[string]string
	}{
		"Labels": {
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"a": "b", "c": "d"},
			},
			want: map[string]string{"a": "b", "c": "d"},
		},
		"Nil": {
			selector: nil,
			want:     map[string]string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := AllocationLabels{
				Selector: tc.selector,
			}

			got := o.GetSelectorLabels()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetFullLabels(t *testing.T) {
	cases := map[string]struct {
		selector *metav1.LabelSelector
		labels   map[string]string
		want     map[string]string
	}{
		"Labels": {
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"a": "b", "c": "d"},
			},
			labels: map[string]string{"e": "f", "g": "h"},
			want:   map[string]string{"a": "b", "c": "d", "e": "f", "g": "h"},
		},
		"Nil": {
			selector: nil,
			want:     map[string]string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := AllocationLabels{
				Selector:          tc.selector,
				UserDefinedLabels: UserDefinedLabels{tc.labels},
			}

			got := o.GetFullLabels()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetLabelSelector(t *testing.T) {
	cases := map[string]struct {
		selector *metav1.LabelSelector
		want     string
	}{
		"Labels": {
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"a": "b", "c": "d"},
			},
			want: "a=b,c=d",
		},
		"Nil": {
			selector: nil,
			want:     "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := AllocationLabels{
				Selector: tc.selector,
			}

			got, err := o.GetLabelSelector()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got.String()); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetOwnerSelector(t *testing.T) {
	cases := map[string]struct {
		labels map[string]string
		want   string
	}{
		"Labels": {
			labels: map[string]string{
				NephioNsnNameKey:           "a",
				NephioNsnNamespaceKey:      "b",
				NephioOwnerGvkKey:          "c",
				NephioOwnerNsnNameKey:      "d",
				NephioOwnerNsnNamespaceKey: "e",
			},
			want: "nephio.org/nsn-name=a,nephio.org/nsn-namespace=b,nephio.org/owner-gvk=c,nephio.org/owner-nsn-name=d,nephio.org/owner-nsn-namespace=e",
		},
		"Nil": {
			want: "nephio.org/nsn-name=,nephio.org/nsn-namespace=,nephio.org/owner-gvk=,nephio.org/owner-nsn-name=,nephio.org/owner-nsn-namespace=",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o := AllocationLabels{
				UserDefinedLabels: UserDefinedLabels{tc.labels},
			}

			got, err := o.GetOwnerSelector()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got.String()); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetOwnerLabelsFromCR(t *testing.T) {
	cases := map[string]struct {
		o    client.Object
		want map[string]string
	}{
		"NoLabels": {
			o: &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "nephio.org/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{},
			},
			want: map[string]string{
				NephioGvkKey:               "Pod.v1alpha1.nephio.org",
				NephioNsnNameKey:           "a",
				NephioNsnNamespaceKey:      "default",
				NephioOwnerGvkKey:          "Pod.v1alpha1.nephio.org",
				NephioOwnerNsnNameKey:      "a",
				NephioOwnerNsnNamespaceKey: "default",
			},
		},
		"Labels": {
			o: &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "nephio.org/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "default",
					Labels: map[string]string{
						NephioOwnerGvkKey:          "A.v1alpha1.nephio.org",
						NephioOwnerNsnNameKey:      "y",
						NephioOwnerNsnNamespaceKey: "x",
					},
				},
				Spec: corev1.PodSpec{},
			},
			want: map[string]string{
				NephioGvkKey:               "Pod.v1alpha1.nephio.org",
				NephioNsnNameKey:           "a",
				NephioNsnNamespaceKey:      "default",
				NephioOwnerGvkKey:          "A.v1alpha1.nephio.org",
				NephioOwnerNsnNameKey:      "y",
				NephioOwnerNsnNamespaceKey: "x",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetHierOwnerLabelsFromCR(tc.o)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}
