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

package meta

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGVKToString(t *testing.T) {
	tests := []struct {
		name string
		gvk  *schema.GroupVersionKind
		want string
	}{
		{
			name: "full gvk input",
			gvk: &schema.GroupVersionKind{
				Group:   "1.2.3.4",
				Version: "v0",
				Kind:    "a",
			},
			want: "a.v0.1.2.3.4",
		},
		{
			name: "empty gvk",
			gvk:  nil,
			want: emptyGvk,
		},
		{
			name: "empty gvk",
			gvk: &schema.GroupVersionKind{
				Group:   "1.2.3.4",
				Version: "v0",
			},
			want: emptyKind,
		},
		{
			name: "empty version",
			gvk: &schema.GroupVersionKind{
				Group: "1.2.3.4",
				Kind:  "a",
			},
			want: "a.1.2.3.4",
		},
		{
			name: "empty group",
			gvk: &schema.GroupVersionKind{
				Kind:    "a",
				Version: "v0",
			},
			want: "a.v0",
		},
		{
			name: "empty group and version",
			gvk: &schema.GroupVersionKind{
				Kind: "a",
			},
			want: "a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GVKToString(tt.gvk); got != tt.want {
				t.Errorf("GVKToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringToGVK(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *schema.GroupVersionKind
	}{
		{
			name:  "full gvk string",
			input: "a.v0.1.2.3.4",
			want: &schema.GroupVersionKind{
				Group:   "1.2.3.4",
				Version: "v0",
				Kind:    "a",
			},
		},
		{
			name:  "empty string",
			input: "",
			want: &schema.GroupVersionKind{
				Group:   "",
				Version: "",
				Kind:    "",
			},
		},
		{
			name:  "kind, empty version/group",
			input: "a",
			want: &schema.GroupVersionKind{
				Group:   "",
				Version: "",
				Kind:    "",
			},
		},
		{
			name:  "kind, emoty group",
			input: "a.v0",
			want: &schema.GroupVersionKind{
				Group:   "",
				Version: "",
				Kind:    "",
			},
		},
		{
			name:  "minimal",
			input: "a.v0.1",
			want: &schema.GroupVersionKind{
				Group:   "1",
				Version: "v0",
				Kind:    "a",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringToGVK(tt.input)
			if got == nil && tt.want == nil {
				return
			}
			if got == nil && tt.want != nil {
				t.Errorf("StringToGVK() = %v, want %v", got, tt.want)
				return
			}
			if got != nil && tt.want == nil {
				t.Errorf("StringToGVK() = %v, want %v", got, tt.want)
				return
			}
			if *got != *tt.want {
				t.Errorf("StringToGVK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApiVersionToGroupVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single /",
			input: "a/b",
			want:  []string{"a", "b"},
		},
		{
			name:  "no /",
			input: "b",
			want:  []string{"", "b"},
		},
		{
			name:  "double /",
			input: "a/b/c",
			want:  []string{"a", "b/c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := apiVersionToGroupVersion(tt.input)
			got := []string{x, y}
			for i := 0; i < 2; i++ {
				if got[i] != tt.want[i] {
					t.Errorf("apiVersionToGroupVersion() = %v, want %v", got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetGVKFromAPIVersionKind(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  *schema.GroupVersionKind
	}{
		{
			name:  "single /",
			input: []string{"a/b", "k"},
			want: &schema.GroupVersionKind{
				Group:   "a",
				Version: "b",
				Kind:    "k",
			},
		},
		{
			name:  "no /",
			input: []string{"a", "k"},
			want: &schema.GroupVersionKind{
				Group:   "",
				Version: "a",
				Kind:    "k",
			},
		},
		{
			name:  "double /",
			input: []string{"a/b/c", "k"},
			want: &schema.GroupVersionKind{
				Group:   "a",
				Version: "b/c",
				Kind:    "k",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetGVKFromAPIVersionKind(tt.input[0], tt.input[1]); *got != *tt.want {
				t.Errorf("GetGVKFromAPIVersionKind() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestGetGVKFromObject(t *testing.T) {
	o := &unstructured.Unstructured{}
	o.SetAPIVersion("1.2.3/v1")
	o.SetKind("a")
	tests := []struct {
		name  string
		input *unstructured.Unstructured
		want  *schema.GroupVersionKind
	}{
		{
			name:  "full gvk input",
			input: o,
			want: &schema.GroupVersionKind{
				Group:   "1.2.3",
				Version: "v1",
				Kind:    "a",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetGVKFromObject(tt.input); *got != *tt.want {
				t.Errorf("GVKToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
