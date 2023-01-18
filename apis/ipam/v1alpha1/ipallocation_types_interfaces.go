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

	//"github.com/hansthienpondt/goipam/pkg/table"
	"github.com/hansthienpondt/nipam/pkg/table"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition of this resource
func (r *IPAllocation) GetCondition(ck ConditionKind) Condition {
	return r.Status.GetCondition(ck)
}

// SetConditions of the Network Node.
func (r *IPAllocation) SetConditions(c ...Condition) {
	r.Status.SetConditions(c...)
}

func (r *IPAllocation) GetNetworkInstance() string {
	return r.Spec.NetworkInstance
}

func (r *IPAllocation) GetPrefixKind() PrefixKind {
	return r.Spec.PrefixKind
}

func (r *IPAllocation) GetPrefix() string {
	return r.Spec.Prefix
}

func (r *IPAllocation) GetCreatePrefix() bool {
	return r.Spec.CreatePrefix
}

/*
func (r *IPAllocation) GetAddressFamily() iputil.AddressFamily {
	switch r.Spec.AddressFamily {
	case "ipv4":
		return iputil.AddressFamilyIpv4
	case "ipv6":
		return iputil.AddressFamilyIpv6
	default:
		return iputil.AddressFamilyUnknown
	}
}
*/

func (x *IPAllocation) GetPrefixLengthFromSpec() iputil.PrefixLength {
	return iputil.PrefixLength(x.Spec.PrefixLength)
}

func (x *IPAllocation) GetPrefixLengthFromRoute(route table.Route) iputil.PrefixLength {
	if x.GetPrefixLengthFromSpec() != 0 {
		return x.GetPrefixLengthFromSpec()
	}
	return iputil.PrefixLength(route.Prefix().Addr().BitLen())
}

func (x *IPAllocation) GetGenericNamespacedName() string {
	return fmt.Sprintf("%s-%s", x.GetNamespace(), x.GetName())
}

func (x *IPAllocation) GetOwnerGvk() string {
	if len(x.Spec.Labels) != 0 {
		return x.Spec.Labels[NephioOwnerGvkKey]
	}
	return ""
}

func (r *IPAllocation) GetSelectorLabels() map[string]string {
	l := map[string]string{}
	if r.Spec.Selector != nil {
		for k, v := range r.Spec.Selector.MatchLabels {
			l[k] = v
		}
	}
	return l
}

func (r *IPAllocation) GetSpecLabels() map[string]string {
	l := map[string]string{}
	for k, v := range r.Spec.Labels {
		l[k] = v
	}
	return l
}

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

func (r *IPAllocation) GetAllocatedPrefix() string {
	return r.Status.AllocatedPrefix
}

func (x *IPAllocation) GetSubnetLabelSelector() (labels.Selector, error) {
	pi, err := iputil.New(x.GetPrefix())
	if err != nil {
		return nil, err
	}
	l := map[string]string{
		NephioSubnetKey: pi.GetSubnetName(),
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

func (r *IPAllocation) GetGatewayLabelSelector() (labels.Selector, error) {
	l := map[string]string{
		NephioGatewayKey: "true",
	}
	fullselector := labels.NewSelector()
	for k, v := range l {
		// exclude any key that is not network and networkinstance
		if k == NephioSubnetKey ||
			//k == ipamv1alpha1.NephioNetworkInstanceKey ||
			k == NephioGatewayKey {
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

func (r *IPAllocation) GetLabelSelector() (labels.Selector, error) {
	l := r.GetSelectorLabels()
	// For prefixkind network we want to allocate only prefixes within a network
	//if r.GetPrefixKind() == PrefixKindNetwork {
	//	l[NephioPrefixKindKey] = string(PrefixKindNetwork)
	//}
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

func (r *IPAllocation) GetOwnerSelector() (labels.Selector, error) {
	l := map[string]string{
		NephioNsnNameKey:           r.Spec.Labels[NephioNsnNameKey],
		NephioNsnNamespaceKey:      r.Spec.Labels[NephioNsnNamespaceKey],
		NephioOwnerGvkKey:          r.Spec.Labels[NephioOwnerGvkKey],
		NephioOwnerNsnNameKey:      r.Spec.Labels[NephioOwnerNsnNameKey],
		NephioOwnerNsnNamespaceKey: r.Spec.Labels[NephioOwnerNsnNamespaceKey],
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

/*
func (r *IPAllocation) GetPrefixFromNewAlloc() string {
	p := r.GetPrefix()
	parentPrefixLength, ok := r.GetSpecLabels()[NephioParentPrefixLengthKey]
	if ok {
		n := strings.Split(p, "/")
		p = strings.Join([]string{n[0], parentPrefixLength}, "/")
	}
	return p
}
*/

func GetPrefixLengthFromAlloc(route table.Route, alloc IPAllocation) uint8 {
	if alloc.Spec.PrefixLength != 0 {
		return alloc.Spec.PrefixLength
	}
	if route.Prefix().Addr().Is4() {
		return 32
	}
	return 128
}

/*
func GetPrefixFromAlloc(p string, alloc *IPAllocation) string {
	parentPrefixLength, ok := alloc.GetSpecLabels()[NephioParentPrefixLengthKey]
	if ok {
		pl, _ := strconv.Atoi(parentPrefixLength)
		pi, err := iputil.New(p)
		if err != nil {
			return ""
		}
		p = pi.GetIPPrefixWithPrefixLength(pl).String()
	}
	return p
}
*/

/*
func GetPrefixFromRoute(route table.Route) string {
	parentPrefixLength := route.Labels().Get(NephioParentPrefixLengthKey)
	p := route.Prefix().String()
	if parentPrefixLength != "" {
		pl, _ := strconv.Atoi(parentPrefixLength)
		pi, err := iputil.New(p)
		if err != nil {
			return ""
		}
		p = pi.GetIPPrefixWithPrefixLength(pl).String()
	}
	return p
}
*/

func BuildIPAllocationFromIPAllocation(cr *IPAllocation) *IPAllocation {
	newcr := cr.DeepCopy()
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)
	// if the ownerGvk is in the labels we use this as ownerGVK
	ownerGVKValue, ok := cr.GetLabels()[NephioOwnerGvkKey]
	if ok {
		ownerGvk = meta.StringToGVK(ownerGVKValue)
	}
	// if the ownerNsn is in the labels we use this as ownerNsn
	ownerNsn := types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}
	ownerNameValue, ok := cr.GetLabels()[NephioOwnerNsnNameKey]
	if ok {
		ownerNsn.Name = ownerNameValue
	}
	ownerNamespaceValue, ok := cr.GetLabels()[NephioOwnerNsnNamespaceKey]
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

func BuildIPAllocationFromIPPrefix(cr *IPPrefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	pi, err := iputil.New(cr.Spec.Prefix)
	if err != nil {
		return nil
	}
	spec := IPAllocationSpec{
		PrefixKind:      cr.Spec.PrefixKind,
		NetworkInstance: cr.Spec.NetworkInstance,
		//AddressFamily:   pi.GetAddressFamily(),
		Prefix:       cr.Spec.Prefix,
		PrefixLength: uint8(pi.GetPrefixLength().Int()),
		CreatePrefix: true,
		Labels: AddSpecLabelsWithTypeMeta(
			ownerGvk,
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
			types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()},
			cr.Spec.Labels), // added the owner label in it
	}
	return BuildIPAllocation(cr.GetNamespace(), cr.GetLabels(), cr.GetName(), spec, IPAllocationStatus{})
}

func BuildIPAllocationFromNetworkInstancePrefix(cr *NetworkInstance, prefix *Prefix) *IPAllocation {
	ownerGvk := meta.GetGVKFromAPIVersionKind(cr.APIVersion, cr.Kind)

	pi, err := iputil.New(prefix.Prefix)
	if err != nil {
		return nil
	}
	spec := IPAllocationSpec{
		PrefixKind:      PrefixKindAggregate,
		NetworkInstance: cr.GetName(),
		//AddressFamily:   pi.GetAddressFamily(),
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
	return BuildIPAllocation(cr.GetNamespace(), cr.GetLabels(), cr.GetNameFromNetworkInstancePrefix(prefix.Prefix), spec, IPAllocationStatus{})
}

func AddSpecLabelsWithTypeMeta(ownerGvk *schema.GroupVersionKind, ownerNsn, nsn types.NamespacedName, specLabels map[string]string) map[string]string {
	labels := map[string]string{
		NephioOwnerGvkKey:          meta.GVKToString(ownerGvk),
		NephioOwnerNsnNameKey:      ownerNsn.Name,
		NephioOwnerNsnNamespaceKey: ownerNsn.Namespace,
		NephioGvkKey:               IPAllocationKindGVKString,
		NephioNsnNameKey:           nsn.Name,
		NephioNsnNamespaceKey:      nsn.Namespace,
	}
	for k, v := range specLabels {
		labels[k] = v
	}
	return labels
}

func BuildIPAllocation(namespace string, labels map[string]string, crName string, spec IPAllocationSpec, status IPAllocationStatus) *IPAllocation {
	return &IPAllocation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: IPAllocationKindAPIVersion,
			Kind:       IPAllocationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      crName,
			Labels:    labels,
		},
		Spec:   spec,
		Status: status,
	}
}

// GetDummyLabelsFromPrefix used in validation
func (r *IPAllocation) GetDummyLabelsFromPrefix(pi iputil.PrefixInfo) map[string]string {
	return map[string]string{
		NephioOwnerGvkKey:          "dummy",
		NephioOwnerNsnNameKey:      "dummy",
		NephioOwnerNsnNamespaceKey: "dummy",
		NephioGvkKey:               IPAllocationKindGVKString,
		NephioNsnNameKey:           r.GetName(),
		NephioNsnNamespaceKey:      r.GetNamespace(),
		NephioPrefixKindKey:        string(r.GetPrefixKind()),
		NephioPrefixLengthKey:      pi.GetPrefixLength().String(),
		NephioSubnetKey:            pi.GetSubnetName(),
	}
}
