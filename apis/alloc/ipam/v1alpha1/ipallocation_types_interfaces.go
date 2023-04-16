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
	"github.com/nokia/k8s-ipam/pkg/iputil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition based on the condition kind
func (r *IPAllocation) GetCondition(ck allocv1alpha1.ConditionKind) allocv1alpha1.Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *IPAllocation) SetConditions(c ...allocv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *IPAllocation) GetGenericNamespacedName() string {
	if r.GetNamespace() == "" {
		return r.GetName()
	}
	return fmt.Sprintf("%s-%s", r.GetNamespace(), r.GetName())
}

// GetPrefixKind returns the prefixkind of the ip allocation
func (r *IPAllocation) GetPrefixKind() PrefixKind {
	return r.Spec.PrefixKind
}

// GetNetworkInstance returns the networkinstance of the ip allocation
func (r *IPAllocation) GetNetworkInstance() corev1.ObjectReference {
	nsn := corev1.ObjectReference{}
	if r.Spec.NetworkInstance != nil {
		nsn.Name = r.Spec.NetworkInstance.Name
		nsn.Namespace = r.Spec.NetworkInstance.Namespace
	}
	return nsn
}

// GetAddressFamily returns the address family of the ip allocation
func (r *IPAllocation) GetAddressFamily() iputil.AddressFamily {
	return r.Spec.AddressFamily
}

// GetPrefix returns the prefix of the ip allocation in cidr notation
// if the prefix was undefined and empty string is returned
func (r *IPAllocation) GetPrefix() string {
	return r.Spec.Prefix
}

// GetPrefixLengthFromSpec returns the prefixlength as defined in the ip allocation spec
func (r *IPAllocation) GetPrefixLengthFromSpec() iputil.PrefixLength {
	return iputil.PrefixLength(r.Spec.PrefixLength)
}

// GetPrefixLengthFromRoute returns the prefixlength of the ip allocation if defined in the spec
// otherwise the route prefix is returned
func (r *IPAllocation) GetPrefixLengthFromRoute(route table.Route) iputil.PrefixLength {
	if r.GetPrefixLengthFromSpec() != 0 {
		return r.GetPrefixLengthFromSpec()
	}
	return iputil.PrefixLength(route.Prefix().Addr().BitLen())
}

// GetIndex returns the index of the ip allocation as defined in the spec
func (r *IPAllocation) GetIndex() uint32 {
	return r.Spec.Index
}

// GetSelector returns the selector of the ip allocation as defined in the spec
// if undefined a nil pointer is returned
func (r *IPAllocation) GetSelector() *metav1.LabelSelector {
	return r.Spec.Selector
}

// GetSelectorLabels returns the matchLabels of the selector as a map[atring]string
// if the selector is undefined an empty map is returned
func (r *IPAllocation) GetSelectorLabels() map[string]string {
	l := map[string]string{}
	if r.Spec.Selector != nil {
		for k, v := range r.Spec.Selector.MatchLabels {
			l[k] = v
		}
	}
	return l
}

// GetSpecLabels returns the labels as defined in the spec in a map
func (r *IPAllocation) GetSpecLabels() map[string]string {
	l := map[string]string{}
	for k, v := range r.Spec.Labels {
		l[k] = v
	}
	return l
}

// GetCreatePrefix return the create prefix flag as defined in the ip allocation spec
func (r *IPAllocation) GetCreatePrefix() bool {
	return r.Spec.CreatePrefix
}

// GetAllocatedPrefix returns the allocated prefix from ip allocation status
func (r *IPAllocation) GetAllocatedPrefix() string {
	return r.Status.AllocatedPrefix
}

// GetGateway returns the gateway from the ip allocation status
func (r *IPAllocation) GetGateway() string {
	return r.Status.Gateway
}

// GetExpiryTime returns the expiry time from the ip allocation status
func (r *IPAllocation) GetExpiryTime() string {
	return r.Status.ExpiryTime
}

// GetOwnerGvk returns the ownergvk as defined in the labels of the ip allocation spec
func (r *IPAllocation) GetOwnerGvk() string {
	if len(r.Spec.Labels) != 0 {
		return r.Spec.Labels[allocv1alpha1.NephioOwnerGvkKey]
	}
	return ""
}

// IsCreatePrefixAllcation validates the IP Allocation and returns an error
// when the validation fails
// When CreatePrefix is set and Prefix is set, the prefix cannot be an address prefix
// When CreatePrefix is set and Prefix is not set, the prefixlength must be defined
// When CreatePrefix is not set and Prefic is set, the prefix must be an address - static address allocation
// When CreatePrefix is not set and Prefic is not set, this is a dynamic address allocation
func (r *IPAllocation) IsCreatePrefixAllcation() (bool, error) {
	// if create prefix is set this is seen as a prefix allocation
	// if create prefix is not set this is seen as an address allocation
	if r.GetCreatePrefix() {
		// create prefix validation
		if r.GetPrefix() != "" {
			pi, err := iputil.New(r.GetPrefix())
			if err != nil {
				return false, err
			}
			if pi.IsAddressPrefix() {
				return false, fmt.Errorf("create prefix is not allowed with /32 or /128, got: %s", pi.GetIPPrefix().String())
			}
			return true, nil
		}
		// this is the case where a dynamic prefix will be allocate based on a prefix length defined by the user
		if r.GetPrefixLengthFromSpec() != 0 {
			return true, nil
		}
	}
	// create address validation
	if r.GetPrefix() != "" {
		pi, err := iputil.New(r.GetPrefix())
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

// GetGatewayLabelSelector returns a label selector to find the gateway of an allocated prefix
// The label selector consists of the selector labels in the spec augmented with the gateway selector
func (r *IPAllocation) GetGatewayLabelSelector() (labels.Selector, error) {
	l := map[string]string{
		allocv1alpha1.NephioGatewayKey: "true",
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		// exclude any key that is not network and networkinstance
		if //k == NephioSubnetKey ||
		//k == ipamv1alpha1.NephioNetworkInstanceKey ||
		k == allocv1alpha1.NephioGatewayKey {
			req, err := labels.NewRequirement(k, selection.Equals, []string{v})
			if err != nil {
				return nil, err
			}
			fullselector = fullselector.Add(*req)
		}
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

// GetLabelSelector returns a labels selector from the selector labels in the spec
func (r *IPAllocation) GetLabelSelector() (labels.Selector, error) {
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

// GetOwnerSelector returns a label selector to find the owner in the ipam backend
func (r *IPAllocation) GetOwnerSelector() (labels.Selector, error) {
	l := map[string]string{
		allocv1alpha1.NephioNsnNameKey:           r.Spec.Labels[allocv1alpha1.NephioNsnNameKey],
		allocv1alpha1.NephioNsnNamespaceKey:      r.Spec.Labels[allocv1alpha1.NephioNsnNamespaceKey],
		allocv1alpha1.NephioOwnerGvkKey:          r.Spec.Labels[allocv1alpha1.NephioOwnerGvkKey],
		allocv1alpha1.NephioOwnerNsnNameKey:      r.Spec.Labels[allocv1alpha1.NephioOwnerNsnNameKey],
		allocv1alpha1.NephioOwnerNsnNamespaceKey: r.Spec.Labels[allocv1alpha1.NephioOwnerNsnNamespaceKey],
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

// GetFullLabels returns a map with a combination of the user defined labels
// in the spec and the selector labels defined in the spec
func (r *IPAllocation) GetFullLabels() map[string]string {
	l := make(map[string]string)
	for k, v := range r.GetSpecLabels() {
		l[k] = v
	}
	for k, v := range r.GetSelectorLabels() {
		l[k] = v
	}
	return l
}

// GetPrefixLengthFromAlloc returns the prefixlength, bu validating the
// prefixlength in the spec. if undefined a /32 or .128 is returned based on the
// prefix in the route
func GetPrefixLengthFromAlloc(route table.Route, alloc IPAllocation) uint8 {
	if alloc.Spec.PrefixLength != 0 {
		return alloc.Spec.PrefixLength
	}
	if route.Prefix().Addr().Is4() {
		return 32
	}
	return 128
}

// BuildIPAllocationFromIPAllocation returns an ip allocation from an ip allocation
// by augmenting the owner GVK/NSN in the user defined labels
func BuildIPAllocationFromIPAllocation(cr *IPAllocation) *IPAllocation {
	newcr := cr.DeepCopy()
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)
	// if the ownerGvk is in the labels we use this as ownerGVK
	ownerGVKValue, ok := cr.GetLabels()[allocv1alpha1.NephioOwnerGvkKey]
	if ok {
		ownerGvk = meta.StringToGVK(ownerGVKValue)
	}
	// if the ownerNsn is in the labels we use this as ownerNsn
	ownerNsn := types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}
	ownerNameValue, ok := cr.GetLabels()[allocv1alpha1.NephioOwnerNsnNameKey]
	if ok {
		ownerNsn.Name = ownerNameValue
	}
	ownerNamespaceValue, ok := cr.GetLabels()[allocv1alpha1.NephioOwnerNsnNamespaceKey]
	if ok {
		ownerNsn.Namespace = ownerNamespaceValue
	}

	newcr.Spec.Labels = AddSpecLabelsWithTypeMeta(
		ownerGvk,
		types.NamespacedName{Namespace: ownerNsn.Namespace, Name: ownerNsn.Name},
		types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
		cr.Spec.Labels,
	) // added the owner label in it
	return newcr
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
		PrefixKind:      cr.Spec.PrefixKind,
		NetworkInstance: cr.Spec.NetworkInstance,
		Prefix:          cr.Spec.Prefix,
		PrefixLength:    uint8(pi.GetPrefixLength().Int()),
		CreatePrefix:    true,
		Labels: AddSpecLabelsWithTypeMeta(
			ownerGvk,
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
			cr.Spec.Labels), // added the owner label in it
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
func BuildIPAllocationFromNetworkInstancePrefix(cr *NetworkInstance, prefix *Prefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	pi, err := iputil.New(prefix.Prefix)
	if err != nil {
		return nil
	}
	spec := IPAllocationSpec{
		PrefixKind: PrefixKindAggregate,
		NetworkInstance: &corev1.ObjectReference{
			Name:      cr.GetName(),
			Namespace: cr.GetNamespace(),
		},
		Prefix:       prefix.Prefix,
		PrefixLength: uint8(pi.GetPrefixLength().Int()),
		CreatePrefix: true,
		Labels: AddSpecLabelsWithTypeMeta(
			ownerGvk,
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetNameFromNetworkInstancePrefix(prefix.Prefix)},
			prefix.Labels), // added the owner label in it
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
func AddSpecLabelsWithTypeMeta(ownerGvk *schema.GroupVersionKind, ownerNsn, nsn types.NamespacedName, specLabels map[string]string) map[string]string {
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

// GetDummyLabelsFromPrefix returns a map with the labels from the spec
// augmented with the prefixkind and the subnet from the prefixInfo
func (r *IPAllocation) GetDummyLabelsFromPrefix(pi iputil.Prefix) map[string]string {
	labels := map[string]string{}
	for k, v := range r.GetSpecLabels() {
		labels[k] = v
	}
	labels[allocv1alpha1.NephioPrefixKindKey] = string(r.GetPrefixKind())
	labels[allocv1alpha1.NephioSubnetKey] = string(pi.GetSubnetName())

	return labels
}
