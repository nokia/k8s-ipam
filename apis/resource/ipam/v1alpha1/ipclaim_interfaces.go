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
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

const IPClaimPlural = "ipclaims"

var _ resource.Object = &IPClaim{}
var _ resource.ObjectList = &IPClaimList{}

// GetCondition returns the condition based on the condition kind
func (r *IPClaim) GetCondition(t resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *IPClaim) SetConditions(c ...resourcev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *IPClaim) GetGenericNamespacedName() string {
	return resourcev1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetCacheID return the cache id validating the namespace
func (r *IPClaim) GetCacheID() corev1.ObjectReference {
	return resourcev1alpha1.GetCacheID(r.Spec.NetworkInstance)
}

// GetPrefixLengthFromRoute returns the prefixlength of the ip claim if defined in the spec
// otherwise the route prefix is returned
func (r *IPClaim) GetPrefixLengthFromRoute(route table.Route) iputil.PrefixLength {
	if r.Spec.PrefixLength != nil {
		return iputil.PrefixLength(*r.Spec.PrefixLength)
	}
	return iputil.PrefixLength(route.Prefix().Addr().BitLen())
}

// GetPrefixLengthFromClaim returns the prefixlength, bu validating the
// prefixlength in the spec. if undefined a /32 or .128 is returned based on the
// prefix in the route
func (r *IPClaim) GetPrefixLengthFromClaim(route table.Route) uint8 {
	if r.Spec.PrefixLength != nil {
		return *r.Spec.PrefixLength
	}
	if route.Prefix().Addr().Is4() {
		return 32
	}
	return 128
}

// GetUserDefinedLabels returns a map with a copy of the user defined labels
func (r *IPClaim) GetUserDefinedLabels() map[string]string {
	return r.Spec.GetUserDefinedLabels()
}

// GetSelectorLabels returns a map with a copy of the selector labels
func (r *IPClaim) GetSelectorLabels() map[string]string {
	return r.Spec.GetSelectorLabels()
}

// GetFullLabels returns a map with a copy of the user defined labels and the selector labels
func (r *IPClaim) GetFullLabels() map[string]string {
	return r.Spec.GetFullLabels()
}

// GetDummyLabelsFromPrefix returns a map with the labels from the spec
// augmented with the prefixkind and the subnet from the prefixInfo
func (r *IPClaim) GetDummyLabelsFromPrefix(pi iputil.Prefix) map[string]string {
	labels := map[string]string{}
	for k, v := range r.GetUserDefinedLabels() {
		labels[k] = v
	}
	labels[resourcev1alpha1.NephioPrefixKindKey] = string(r.Spec.Kind)
	labels[resourcev1alpha1.NephioSubnetKey] = string(pi.GetSubnetName())

	return labels
}

// GetLabelSelector returns a labels selector based on the label selector
func (r *IPClaim) GetLabelSelector() (labels.Selector, error) {
	return r.Spec.GetLabelSelector()
}

// GetOwnerSelector returns a label selector to select the owner of the claim in the backend
func (r *IPClaim) GetOwnerSelector() (labels.Selector, error) {
	return r.Spec.GetOwnerSelector()
}

// GetGatewayLabelSelector returns a label selector to select the gateway of the claim in the backend
func (r *IPClaim) GetGatewayLabelSelector(subnetString string) (labels.Selector, error) {
	l := map[string]string{
		resourcev1alpha1.NephioGatewayKey: "true",
		resourcev1alpha1.NephioSubnetKey:  subnetString,
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

// IsCreatePrefixAllcationValid validates the IP Claim and returns an error
// when the validation fails
// When CreatePrefix is set and Prefix is set, the prefix cannot be an address prefix
// When CreatePrefix is set and Prefix is not set, the prefixlength must be defined
// When CreatePrefix is not set and Prefix is set, the prefix must be an address - static address claim
// When CreatePrefix is not set and Prefix is not set, this is a dynamic address claim
func (r *IPClaim) IsCreatePrefixAllcationValid() (bool, error) {
	// if create prefix is set this is seen as a prefix claim
	// if create prefix is not set this is seen as an address claim
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
		// this is the case where a dynamic prefix will be claimed based on a prefix length defined by the user
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

// AddOwnerLabelsToCR returns a VLANClaim
// by augmenting the owner GVK/NSN in the user defined labels
func (r *IPClaim) AddOwnerLabelsToCR() {
	if r.Spec.UserDefinedLabels.Labels == nil {
		r.Spec.UserDefinedLabels.Labels = map[string]string{}
	}
	for k, v := range resourcev1alpha1.GetHierOwnerLabelsFromCR(r) {
		r.Spec.UserDefinedLabels.Labels[k] = v
	}
}

// BuildIPClaim returns an IP Claim from a client Object a crName and
// an IPClaim Spec/Status
func BuildIPClaim(meta metav1.ObjectMeta, spec IPClaimSpec, status IPClaimStatus) *IPClaim {
	return &IPClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       IPClaimKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}

func (IPClaim) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: IPClaimPlural,
	}
}

// IsStorageVersion returns true -- v1alpha1.Config is used as the internal version.
// IsStorageVersion implements resource.Object.
func (IPClaim) IsStorageVersion() bool {
	return true
}

// GetObjectMeta implements resource.Object
func (r *IPClaim) GetObjectMeta() *metav1.ObjectMeta {
	return &r.ObjectMeta
}

// NamespaceScoped returns true to indicate Fortune is a namespaced resource.
// NamespaceScoped implements resource.Object.
func (IPClaim) NamespaceScoped() bool {
	return true
}

// New implements resource.Object
func (IPClaim) New() runtime.Object {
	return &IPClaim{}
}

// NewList implements resource.Object
func (IPClaim) NewList() runtime.Object {
	return &IPClaimList{}
}

// GetListMeta returns the ListMeta
func (r *IPClaimList) GetListMeta() *metav1.ListMeta {
	return &r.ListMeta
}
