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

package ipam

import (
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/utils/iputil"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type selectors struct {
	fullSelector    labels.Selector
	labelSelector   labels.Selector
	allocSelector   labels.Selector
	gatewaySelector labels.Selector
	labels          map[string]string
}

func GetNetworkLabelSelector(cr *ipamv1alpha1.IPPrefix) (labels.Selector, error) {
	fullselector := labels.NewSelector()

	p := netaddr.MustParseIPPrefix(cr.Spec.Prefix)
	af := iputil.GetAddressFamily(p)

	req, err := labels.NewRequirement(ipamv1alpha1.NephioNetworkNameKey, selection.In, []string{cr.Spec.Network})
	if err != nil {
		return nil, err
	}
	fullselector = fullselector.Add(*req)
	req, err = labels.NewRequirement(ipamv1alpha1.NephioAddressFamilyKey, selection.In, []string{string(af)})
	if err != nil {
		return nil, err
	}
	fullselector = fullselector.Add(*req)

	return fullselector, nil
}

func getGatewaySelector(alloc *ipamv1alpha1.IPAllocation) (labels.Selector, error) {
	fullselector := labels.NewSelector()
	for key, val := range alloc.Spec.Selector.MatchLabels {
		req, err := labels.NewRequirement(key, selection.In, []string{val})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	// For prefixkind network we want to allocate only prefixes within a network
	if alloc.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindNetwork) {
		req, err := labels.NewRequirement(ipamv1alpha1.NephioPrefixKindKey, selection.In, []string{string(ipamv1alpha1.PrefixKindNetwork)})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
		req, err = labels.NewRequirement(ipamv1alpha1.NephioGatewayKey, selection.In, []string{"true"})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func getLabelSelector(alloc *ipamv1alpha1.IPAllocation) (labels.Selector, error) {
	fullselector := labels.NewSelector()
	for key, val := range alloc.Spec.Selector.MatchLabels {
		req, err := labels.NewRequirement(key, selection.In, []string{val})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	// For prefixkind network we want to allocate only prefixes within a network
	if alloc.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindNetwork) {
		req, err := labels.NewRequirement(ipamv1alpha1.NephioPrefixKindKey, selection.In, []string{string(ipamv1alpha1.PrefixKindNetwork)})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func getAllocSelector(alloc *ipamv1alpha1.IPAllocation) (labels.Selector, error) {
	fullselector := labels.NewSelector()
	req, err := labels.NewRequirement(ipamv1alpha1.NephioIPAllocactionNameKey, selection.In, []string{alloc.GetName()})
	if err != nil {
		return nil, err
	}
	return fullselector.Add(*req), nil
}

// getSelectors returns the labelSelector and the full selector
func getSelectors(alloc *ipamv1alpha1.IPAllocation) (*selectors, error) {
	labelSelector, err := getLabelSelector(alloc)
	if err != nil {
		return nil, err
	}

	allocSelector, err := getAllocSelector(alloc)
	if err != nil {
		return nil, err
	}

	gatewaySelector, err := getGatewaySelector(alloc)
	if err != nil {
		return nil, err
	}

	fullselector := labels.NewSelector()
	l := make(map[string]string)
	for key, val := range alloc.Spec.Selector.MatchLabels {
		req, err := labels.NewRequirement(key, selection.In, []string{val})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
		l[key] = val
	}
	for key, val := range alloc.GetLabels() {
		req, err := labels.NewRequirement(key, selection.In, []string{val})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
		l[key] = val
	}
	// add the allocation Name key to the label list as we use this internally
	// for validating prefix names
	l[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	return &selectors{
		fullSelector:    fullselector,
		labelSelector:   labelSelector,
		allocSelector:   allocSelector,
		gatewaySelector: gatewaySelector,
		labels:          l,
	}, nil
}
