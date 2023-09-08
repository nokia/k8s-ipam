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
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition based on the condition kind
func (r *VXLANClaim) GetCondition(t resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *VXLANClaim) SetConditions(c ...resourcev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *VXLANClaim) GetGenericNamespacedName() string {
	return resourcev1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetCacheID return the cache id validating the namespace
func (r *VXLANClaim) GetCacheID() corev1.ObjectReference {
	return resourcev1alpha1.GetCacheID(r.Spec.VXLANIndex)
}

// GetUserDefinedLabels returns a map with a copy of the user defined labels
func (r *VXLANClaim) GetUserDefinedLabels() map[string]string {
	return r.Spec.GetUserDefinedLabels()
}

// GetSelectorLabels returns a map with a copy of the selector labels
func (r *VXLANClaim) GetSelectorLabels() map[string]string {
	return r.Spec.GetSelectorLabels()
}

// GetFullLabels returns a map with a copy of the user defined labels and the selector labels
func (r *VXLANClaim) GetFullLabels() map[string]string {
	return r.Spec.GetFullLabels()
}

// GetLabelSelector returns a labels selector based on the label selector
func (r *VXLANClaim) GetLabelSelector() (labels.Selector, error) {
	return r.Spec.GetLabelSelector()
}

// GetOwnerSelector returns a label selector to select the owner of the claim in the backend
func (r *VXLANClaim) GetOwnerSelector() (labels.Selector, error) {
	return r.Spec.GetOwnerSelector()
}

// AddOwnerLabelsToCR returns a VXLAN Claim
// by augmenting the owner GVK/NSN in the user defined labels
func (r *VXLANClaim) AddOwnerLabelsToCR() {
	if r.Spec.UserDefinedLabels.Labels == nil {
		r.Spec.UserDefinedLabels.Labels = map[string]string{}
	}
	for k, v := range resourcev1alpha1.GetHierOwnerLabelsFromCR(r) {
		r.Spec.UserDefinedLabels.Labels[k] = v
	}
}

// BuildVXLANClaim returns a VXLANClaim from a client Object a crName and
// an VXLANClaim Spec/Status
func BuildVXLANClaim(meta metav1.ObjectMeta, spec VXLANClaimSpec, status VXLANClaimStatus) *VXLANClaim {
	return &VXLANClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       VXLANClaimKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}

func (r *VXLANClaim) GetVXLANClaimCtx() (*VXLANClaimCtx, error) {
	return &VXLANClaimCtx{Kind: VXLANClaimTypeDynamic}, nil
}

type VXLANClaimCtx struct {
	Kind VXLANClaimType
	//Start uint16
	//Size  uint16
}

type VXLANClaimType string

const (
	VXLANClaimTypeDynamic VXLANClaimType = "dynamic"
	//VLANClaimTypeStatic  VLANClaimType = "static"
	//VLANClaimTypeSize    VLANClaimType = "size"
	//VLANClaimTypeRange   VLANClaimType = "range"
)
