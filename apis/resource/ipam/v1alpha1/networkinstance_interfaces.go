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
	"strings"

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	NetworkInstancePrefixAggregate = "aggregate"
)

// GetCondition returns the condition based on the condition kind
func (r *NetworkInstance) GetCondition(t resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *NetworkInstance) SetConditions(c ...resourcev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

// GetNamespacedName returns the namespace and name
func (r *NetworkInstance) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

// GetNameFromNetworkInstancePrefix return a network instance prefix name
// It provides a unique name by concatenating the networkinstance name
// with a predefined name (aggregate) and a prefix, compliant to the naming
// convention of k8s
func (r *NetworkInstance) GetNameFromNetworkInstancePrefix(prefix string) string {
	return GetNameFromPrefix(prefix, r.GetName(), NetworkInstancePrefixAggregate)
}

func GetNameFromPrefix(prefix, name, suffix string) string {
	s := fmt.Sprintf("%s-%s", strings.ReplaceAll(prefix, "/", "-"), name)
	s = strings.ReplaceAll(s, ":", "-")
	if suffix != "" {
		s = fmt.Sprintf("%s-%s", s, suffix)
	}
	return s
}

// GetGenericNamespacedName return a namespace and name
// as string, compliant to the k8s api naming convention
func (r *NetworkInstance) GetGenericNamespacedName() string {
	return resourcev1alpha1.GetGenericNamespacedName(types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
	})
}

// GetCacheID returns a CacheID as an objectReference
func (r *NetworkInstance) GetCacheID() corev1.ObjectReference {
	return resourcev1alpha1.GetCacheID(corev1.ObjectReference{Name: r.GetName(), Namespace: r.GetNamespace()})
}

// BuildNetworkInstance returns a networkInstance based on meta, spec and statis
func BuildNetworkInstance(meta metav1.ObjectMeta, spec NetworkInstanceSpec, status NetworkInstanceStatus) *NetworkInstance {
	return &NetworkInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeBuilder.GroupVersion.Identifier(),
			Kind:       NetworkInstanceKind,
		},
		ObjectMeta: meta,
		Spec:       spec,
		Status:     status,
	}
}

func (r *Prefix) GetPrefixKind() PrefixKind {
	prefixKind := PrefixKindNetwork
	if value, ok := r.Labels[resourcev1alpha1.NephioPrefixKindKey]; ok {
		prefixKind = GetPrefixKindFromString(value)
	}
	return prefixKind
}

type PrefixClaimContext struct {
	PrefixClaimName         string
	PrefixSelectorLabels    map[string]string
	PrefixUserDefinedLabels map[string]string

	AddressClaimCOntext
}

type AddressClaimCOntext struct {
	AddressClaimName         string
	AddressSelectorLabels    map[string]string
	AddressUserDefinedLabels map[string]string
}

type Labels map[string]string

func (r Labels) GetPurposeLabels(purpose string) map[string]string {
	newLabels := map[string]string{}
	for k, v := range r {
		newLabels[k] = v
	}
	newLabels[resourcev1alpha1.NephioPurposeKey] = purpose
	return newLabels
}

// GetPrefixClaimContext return selectors and user defined labels
// the claim is based on prefix namings used internally and is hierarchical
// for irb there is a special case since the top prefix is claimed in the routing context
// the 2nd prefix get a name of the bridge context. As such we have rtNmame and bdName
func (r *Prefix) GetPrefixClaimContext(rtName, bdName, linkName, nodeName, namespace string, labels Labels) *PrefixClaimContext {
	prefixClaimName := GetNameFromPrefix(r.Prefix, rtName, NetworkInstancePrefixAggregate)
	linkPrefixClaimName := GetNameFromPrefix(r.Prefix, bdName, linkName)
	linkAddressPrefixClaimName := fmt.Sprintf("%s-%s", linkPrefixClaimName, nodeName)

	return &PrefixClaimContext{
		PrefixClaimName: linkPrefixClaimName,
		PrefixSelectorLabels: map[string]string{
			resourcev1alpha1.NephioNsnNameKey:      prefixClaimName,
			resourcev1alpha1.NephioNsnNamespaceKey: namespace,
		},
		PrefixUserDefinedLabels: labels.GetPurposeLabels("link prefix"),

		AddressClaimCOntext: AddressClaimCOntext{
			AddressClaimName: linkAddressPrefixClaimName,
			AddressSelectorLabels: map[string]string{
				resourcev1alpha1.NephioNsnNameKey:      linkPrefixClaimName,
				resourcev1alpha1.NephioNsnNamespaceKey: namespace,
			},
			AddressUserDefinedLabels: map[string]string{
				resourcev1alpha1.NephioGatewayKey: "true",
				resourcev1alpha1.NephioPurposeKey: "link address",
			},
		},
	}
}

func (r *Prefix) GetPrefixSelectorLabels(name, namespace string) map[string]string {
	return map[string]string{
		resourcev1alpha1.NephioNsnNameKey:      GetNameFromPrefix(r.Prefix, name, NetworkInstancePrefixAggregate),
		resourcev1alpha1.NephioNsnNamespaceKey: namespace,
	}
}

func (r *Prefix) GetAddressSelectorLabels(name, namespace string) map[string]string {
	return map[string]string{
		resourcev1alpha1.NephioNsnNameKey:      name,
		resourcev1alpha1.NephioNsnNamespaceKey: namespace,
	}
}

func (r *Prefix) GetPrefixUserDefinedLabels(labels map[string]string, value string) map[string]string {
	labels[resourcev1alpha1.NephioPurposeKey] = value
	return labels
}

func (r *Prefix) GetAddressClaimContext(rtName, nodeName, namespace string, labels Labels) *AddressClaimCOntext {
	prefixClaimName := GetNameFromPrefix(r.Prefix, rtName, NetworkInstancePrefixAggregate)
	addressClaimName := GetNameFromPrefix(r.Prefix, nodeName, "system")
	return &AddressClaimCOntext{
		AddressClaimName: addressClaimName,
		AddressSelectorLabels: map[string]string{
			resourcev1alpha1.NephioNsnNameKey:      prefixClaimName,
			resourcev1alpha1.NephioNsnNamespaceKey: namespace,
		},
		AddressUserDefinedLabels: map[string]string{
			invv1alpha1.NephioInventoryNodeNameKey: nodeName,
			resourcev1alpha1.NephioPurposeKey:      "node loopback address",
		},
	}
}
