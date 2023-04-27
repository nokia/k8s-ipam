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
	"github.com/nokia/k8s-ipam/internal/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UserDefinedLabels define metadata to the resource.
type UserDefinedLabels struct {
	// Labels as user defined labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// GetUserDefinedLabels returns a map with a copy of the
// user defined labels
func (r *UserDefinedLabels) GetUserDefinedLabels() map[string]string {
	l := map[string]string{}
	if len(r.Labels) != 0 {
		for k, v := range r.Labels {
			l[k] = v
		}
	}
	return l
}

type AllocationLabels struct {
	UserDefinedLabels
	// Selector defines the selector criterias for the VLAN allocation
	// +kubebuilder:validation:Optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
}

// GetUserDefinedLabels returns a map with a copy of the
// user defined labels
func (r *AllocationLabels) GetUserDefinedLabels() map[string]string {
	return r.UserDefinedLabels.GetUserDefinedLabels()
}

// GetSelectorLabels returns a map with a copy of the
// selector labels
func (r *AllocationLabels) GetSelectorLabels() map[string]string {
	l := map[string]string{}
	if r.Selector != nil {
		for k, v := range r.Selector.MatchLabels {
			l[k] = v
		}
	}
	return l
}

// GetFullLabels returns a map with a copy of the
// user defined labels and the selector labels
func (r *AllocationLabels) GetFullLabels() map[string]string {
	l := make(map[string]string)
	for k, v := range r.GetUserDefinedLabels() {
		l[k] = v
	}
	for k, v := range r.GetSelectorLabels() {
		l[k] = v
	}
	return l
}

// GetLabelSelector returns a labels selector based
// on the label selector
func (r *AllocationLabels) GetLabelSelector() (labels.Selector, error) {
	l := r.GetSelectorLabels()
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

// GetOwnerSelector returns a label selector to select the owner
// of the allocation in the backend
func (r *AllocationLabels) GetOwnerSelector() (labels.Selector, error) {
	l := map[string]string{
		NephioNsnNameKey:           r.Labels[NephioNsnNameKey],
		NephioNsnNamespaceKey:      r.Labels[NephioNsnNamespaceKey],
		NephioOwnerGvkKey:          r.Labels[NephioOwnerGvkKey],
		NephioOwnerNsnNameKey:      r.Labels[NephioOwnerNsnNameKey],
		NephioOwnerNsnNamespaceKey: r.Labels[NephioOwnerNsnNamespaceKey],
	}

	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func GetOwnerLabelsFromCR(cr client.Object) map[string]string {
	// if the ownerGvk is in the labels we use this as ownerGVK
	crGVK := cr.GetObjectKind().GroupVersionKind()
	ownerGVKValue, ok := cr.GetLabels()[NephioOwnerGvkKey]
	if !ok {
		ownerGVK := cr.GetObjectKind().GroupVersionKind()
		ownerGVKValue = meta.GVKToString(ownerGVK)
	}
	// if the ownerNSN is in the labels we use this as ownerNSN
	ownerNSN := types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}
	ownerNameValue, ok := cr.GetLabels()[NephioOwnerNsnNameKey]
	if ok {
		ownerNSN.Name = ownerNameValue
	}
	ownerNamespaceValue, ok := cr.GetLabels()[NephioOwnerNsnNamespaceKey]
	if ok {
		ownerNSN.Namespace = ownerNamespaceValue
	}

	return map[string]string{
		NephioOwnerGvkKey:          ownerGVKValue,
		NephioOwnerNsnNameKey:      ownerNSN.Name,
		NephioOwnerNsnNamespaceKey: ownerNSN.Namespace,
		NephioGvkKey:               meta.GVKToString(crGVK),
		NephioNsnNameKey:           cr.GetName(),
		NephioNsnNamespaceKey:      cr.GetNamespace(),
	}
}
