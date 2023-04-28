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
	"fmt"

	"github.com/hansthienpondt/nipam/pkg/table"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/utils/util"
	"github.com/nokia/k8s-ipam/pkg/iputil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

// GetCondition returns the condition based on the condition kind
func (r *IPAllocation) GetCondition(t allocv1alpha1.ConditionType) allocv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *IPAllocation) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *IPAllocation) GetGenericNamespacedName() string {
	return allocv1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetCacheID return the cache id validating the namespace
func (r *IPAllocation) GetCacheID() corev1.ObjectReference {
	namespace := r.Spec.NetworkInstance.Namespace
	if namespace == "" {
		namespace = r.GetNamespace()
	}
	return corev1.ObjectReference{Name: r.Spec.NetworkInstance.Name, Namespace: namespace}
}

// GetPrefixLengthFromRoute returns the prefixlength of the ip allocation if defined in the spec
// otherwise the route prefix is returned
func (r *IPAllocation) GetPrefixLengthFromRoute(route table.Route) iputil.PrefixLength {
	if r.Spec.PrefixLength != nil {
		return iputil.PrefixLength(*r.Spec.PrefixLength)
	}
	return iputil.PrefixLength(route.Prefix().Addr().BitLen())
}

// GetPrefixLengthFromAlloc returns the prefixlength, bu validating the
// prefixlength in the spec. if undefined a /32 or .128 is returned based on the
// prefix in the route
func (r *IPAllocation) GetPrefixLengthFromAlloc(route table.Route) uint8 {
	if r.Spec.PrefixLength != nil {
		return *r.Spec.PrefixLength
	}
	if route.Prefix().Addr().Is4() {
		return 32
	}
	return 128
}

// GetUserDefinedLabels returns a map with a copy of the user defined labels
func (r *IPAllocation) GetUserDefinedLabels() map[string]string {
	return r.Spec.GetUserDefinedLabels()
}

// GetSelectorLabels returns a map with a copy of the selector labels
func (r *IPAllocation) GetSelectorLabels() map[string]string {
	return r.Spec.GetSelectorLabels()
}

// GetFullLabels returns a map with a copy of the user defined labels and the selector labels
func (r *IPAllocation) GetFullLabels() map[string]string {
	return r.Spec.GetFullLabels()
}

// GetDummyLabelsFromPrefix returns a map with the labels from the spec
// augmented with the prefixkind and the subnet from the prefixInfo
func (r *IPAllocation) GetDummyLabelsFromPrefix(pi iputil.Prefix) map[string]string {
	labels := map[string]string{}
	for k, v := range r.GetUserDefinedLabels() {
		labels[k] = v
	}
	labels[allocv1alpha1.NephioPrefixKindKey] = string(r.Spec.Kind)
	labels[allocv1alpha1.NephioSubnetKey] = string(pi.GetSubnetName())

	return labels
}

// GetLabelSelector returns a labels selector based on the label selector
func (r *IPAllocation) GetLabelSelector() (labels.Selector, error) {
	return r.Spec.GetLabelSelector()
}

// GetOwnerSelector returns a label selector to select the owner of the allocation in the backend
func (r *IPAllocation) GetOwnerSelector() (labels.Selector, error) {
	return r.Spec.GetOwnerSelector()
}

// GetGatewayLabelSelector returns a label selector to select the gateway of the allocation in the backend
func (r *IPAllocation) GetGatewayLabelSelector() (labels.Selector, error) {
	l := map[string]string{
		allocv1alpha1.NephioGatewayKey: "true",
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	for k, v := range r.GetSelectorLabels() {
		req, err := labels.NewRequirement(k, selection.In, []string{v})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

// IsCreatePrefixAllcationValid validates the IP Allocation and returns an error
// when the validation fails
// When CreatePrefix is set and Prefix is set, the prefix cannot be an address prefix
// When CreatePrefix is set and Prefix is not set, the prefixlength must be defined
// When CreatePrefix is not set and Prefix is set, the prefix must be an address - static address allocation
// When CreatePrefix is not set and Prefix is not set, this is a dynamic address allocation
func (r *IPAllocation) IsCreatePrefixAllcationValid() (bool, error) {
	// if create prefix is set this is seen as a prefix allocation
	// if create prefix is not set this is seen as an address allocation
	if r.Spec.CreatePrefix != nil {
		// create prefix validation
		if r.Spec.Prefix != nil {
			pi, err := iputil.New(*r.Spec.Prefix)
			if err != nil {
				return false, err
			}
			if pi.IsAddressPrefix() {
				return false, fmt.Errorf("create prefix is not allowed with /32 or /128, got: %s", pi.GetIPPrefix().String())
			}
			return true, nil
		}
		// this is the case where a dynamic prefix will be allocate based on a prefix length defined by the user
		if r.Spec.PrefixLength != nil {
			return true, nil
		}
		return false, fmt.Errorf("create prefix needs a prefixLength or prefix got none")

	}
	// create address validation
	if r.Spec.Prefix != nil {
		pi, err := iputil.New(*r.Spec.Prefix)
		if err != nil {
			return false, err
		}
		if !pi.IsAddressPrefix() {
			return false, fmt.Errorf("create address is only allowed with /32 or .128, got: %s", pi.GetIPPrefix().String())
		}
		return false, nil
	}
	return false, nil
}

// AddOwnerLabelsToCR returns a VLANAllocation
// by augmenting the owner GVK/NSN in the user defined labels
func (r *IPAllocation) AddOwnerLabelsToCR() {
	if r.Spec.UserDefinedLabels.Labels == nil {
		r.Spec.UserDefinedLabels.Labels = map[string]string{}
	}
	for k, v := range allocv1alpha1.GetOwnerLabelsFromCR(r) {
		r.Spec.UserDefinedLabels.Labels[k] = v
	}
}

// BuildIPAllocationFromIPPrefix returns an IP allocation from an IP Prefix
// by augmenting the owner GVK/NSN in the user defined labels and populating
// the information from the IP prefix in the IP Allocation spec attributes.
func BuildIPAllocationFromIPPrefix(cr *IPPrefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	pi, err := iputil.New(cr.Spec.Prefix)
	if err != nil {
		return nil
	}
	spec := IPAllocationSpec{
		Kind:            cr.Spec.Kind,
		NetworkInstance: cr.Spec.NetworkInstance,
		Prefix:          &cr.Spec.Prefix,
		PrefixLength:    util.PointerUint8(pi.GetPrefixLength().Int()),
		CreatePrefix:    pointer.Bool(true),
		AllocationLabels: allocv1alpha1.AllocationLabels{
			UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
				Labels: AddSpecLabelsWithTypeMeta(
					ownerGvk,
					types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
					types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
					cr.Spec.Labels), // added the owner label in it
			},
		},
	}
	meta := metav1.ObjectMeta{
		Name:      cr.GetName(),
		Namespace: cr.GetNamespace(),
		Labels:    cr.GetLabels(),
	}
	return BuildIPAllocation(meta, spec, IPAllocationStatus{})
}

// BuildIPAllocationFromNetworkInstancePrefix returns an IP allocation from an NetworkInstance prefix
// by augmenting the owner GVK/NSN in the user defined labels and populating
// the information from the IP prefix in the IP Allocation spec attributes.
func BuildIPAllocationFromNetworkInstancePrefix(cr *NetworkInstance, prefix Prefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	pi, err := iputil.New(prefix.Prefix)
	if err != nil {
		return nil
	}
	spec := IPAllocationSpec{
		Kind: PrefixKindAggregate,
		NetworkInstance: corev1.ObjectReference{
			Name:      cr.GetName(),
			Namespace: cr.GetNamespace(),
		},
		Prefix:       &prefix.Prefix,
		PrefixLength: util.PointerUint8(pi.GetPrefixLength().Int()),
		CreatePrefix: pointer.Bool(true),
		AllocationLabels: allocv1alpha1.AllocationLabels{
			UserDefinedLabels: allocv1alpha1.UserDefinedLabels{
				Labels: AddSpecLabelsWithTypeMeta(
					ownerGvk,
					types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
					types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetNameFromNetworkInstancePrefix(prefix.Prefix)},
					prefix.GetUserDefinedLabels()), // added the owner label in it
			},
		},
	}
	// name is based on aggregate and prefix
	meta := metav1.ObjectMeta{
		Name:      cr.GetNameFromNetworkInstancePrefix(prefix.Prefix),
		Namespace: cr.GetNamespace(),
		Labels:    cr.GetLabels(),
	}
	return BuildIPAllocation(meta, spec, IPAllocationStatus{})
}

// AddSpecLabelsWithTypeMeta returns a map based on the owner GVK/NSN
func AddSpecLabelsWithTypeMeta(ownerGvk schema.GroupVersionKind, ownerNsn, nsn types.NamespacedName, specLabels map[string]string) map[string]string {
	labels := map[string]string{
		allocv1alpha1.NephioOwnerGvkKey:          meta.GVKToString(ownerGvk),
		allocv1alpha1.NephioOwnerNsnNameKey:      ownerNsn.Name,
		allocv1alpha1.NephioOwnerNsnNamespaceKey: ownerNsn.Namespace,
		allocv1alpha1.NephioGvkKey:               IPAllocationKindGVKString,
		allocv1alpha1.NephioNsnNameKey:           nsn.Name,
		allocv1alpha1.NephioNsnNamespaceKey:      nsn.Namespace,
	}
	for k, v := range specLabels {
		labels[k] = v
	}
	return labels
}

// BuildIPAllocation returns an IP Allocation from a client Object a crName and
// an IPAllocation Spec/Status
func BuildIPAllocation(meta metav1.ObjectMeta, spec IPAllocationSpec, status IPAllocationStatus) *IPAllocation {
	return &IPAllocation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       IPAllocationKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}
