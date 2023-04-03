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
	"context"
	"fmt"
	"strings"
	"testing"

	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type allocation struct {
	kind      string
	name      string
	namespace string
	spec      ipamv1alpha1.IPAllocationSpec
}

func buildNetworkInstance(alloc *allocation) *ipamv1alpha1.NetworkInstance {
	return &ipamv1alpha1.NetworkInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ipamv1alpha1.GroupVersion.String(),
			Kind:       ipamv1alpha1.NetworkInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: alloc.spec.NetworkInstance.Namespace,
			Name:      alloc.spec.NetworkInstance.Name,
		},
	}
}

func buildIPAllocation(alloc *allocation) *ipamv1alpha1.IPAllocation {
	switch alloc.kind {
	case ipamv1alpha1.NetworkInstanceKind:
		return ipamv1alpha1.BuildIPAllocationFromNetworkInstancePrefix(
			&ipamv1alpha1.NetworkInstance{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ipamv1alpha1.GroupVersion.String(),
					Kind:       ipamv1alpha1.NetworkInstanceKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alloc.namespace,
					Name:      alloc.name,
				},
			},
			&ipamv1alpha1.Prefix{
				Prefix: alloc.spec.Prefix,
				Labels: alloc.spec.Labels,
			},
		)
	case ipamv1alpha1.IPPrefixKind:
		return ipamv1alpha1.BuildIPAllocationFromIPPrefix(
			&ipamv1alpha1.IPPrefix{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ipamv1alpha1.GroupVersion.String(),
					Kind:       ipamv1alpha1.IPPrefixKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alloc.namespace,
					Name:      alloc.name,
				},
				Spec: ipamv1alpha1.IPPrefixSpec{
					NetworkInstance: alloc.spec.NetworkInstance,
					PrefixKind:      alloc.spec.PrefixKind,
					Prefix:          alloc.spec.Prefix,
					Labels:          alloc.spec.Labels,
				},
			},
		)
	case ipamv1alpha1.IPAllocationKind:
		return ipamv1alpha1.BuildIPAllocationFromIPAllocation(
			&ipamv1alpha1.IPAllocation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ipamv1alpha1.GroupVersion.String(),
					Kind:       ipamv1alpha1.IPAllocationKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alloc.namespace,
					Name:      alloc.name,
				},
				Spec: alloc.spec,
			},
		)
	}
	return nil
}

func TestNetworkInstance(t *testing.T) {
	niRef := &corev1.ObjectReference{
		Namespace: "dummy",
		Name:      "niName",
	}
	allocNamespace := "test"
	niCreate := &allocation{spec: ipamv1alpha1.IPAllocationSpec{NetworkInstance: niRef}}

	type ipamTests struct {
		allocation *allocation
		errString  string
		gateway    string
	}

	tests := []ipamTests{
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.NetworkInstanceKind,
				namespace: niRef.Namespace,
				name:      niRef.Name,
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstance: niRef,
					Prefix:          "10.0.0.0/8",
				},
			},
			errString: "",
		},
		// create network prefix w/o parent
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstance: niRef,
					PrefixKind:      ipamv1alpha1.PrefixKindNetwork,
					Prefix:          "10.0.0.2/24",
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: errValidateNetworkPrefixWoNetworkParent,
		},
		// create parent prefix
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstance: niRef,
					PrefixKind:      ipamv1alpha1.PrefixKindNetwork,
					Prefix:          "10.0.0.1/24",
					CreatePrefix:    true,
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: "",
		},
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-staticprefix",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstance: niRef,
					PrefixKind:      ipamv1alpha1.PrefixKindNetwork,
					Prefix:          "10.0.0.2/24",
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: errValidateNetworkPrefixWoNetworkParent,
		},
		// test duplication
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix2",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstance: niRef,
					PrefixKind:      ipamv1alpha1.PrefixKindNetwork,
					Prefix:          "10.0.0.10/24",
					CreatePrefix:    true,
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			},
			errString: errValidateDuplicatePrefix,
		},
		// test allocation
		{
			allocation: &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-alloc1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstance: niRef,
					PrefixKind:      ipamv1alpha1.PrefixKindNetwork,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nephio.org/region":       "us-central1",
							"nephio.org/site":         "edge1",
							"nephio.org/network-name": "net1",
						},
					},
				},
			},
			errString: "",
			gateway:   "10.0.0.1",
		},
	}

	// create new rib
	ipam := New(nil)
	// create new networkinstance
	niCr := buildNetworkInstance(niCreate)
	if err := ipam.Create(context.Background(), niCr); err != nil {
		t.Errorf("%v occured, cannot create network instance: %s/%s", err, niCr.GetNamespace(), niCr.GetName())
	}

	for _, test := range tests {
		allocReq := buildIPAllocation(test.allocation)
		allocResp, err := ipam.AllocateIPPrefix(context.Background(), allocReq)
		if err != nil {
			if test.errString == "" {
				t.Errorf("%v, cannot create ip prefix: %v", err, allocResp)
				return
			} else {
				// expected error
				if !strings.Contains(err.Error(), test.errString) {
					t.Errorf("expecting error: %s, got %v resp: %v", test.errString, err, allocResp)
					return
				}
			}
		}
		if test.errString == "" {
			if test.allocation.spec.Prefix != "" {
				if allocResp.Status.AllocatedPrefix != test.allocation.spec.Prefix {
					t.Errorf("expected prefix %s, got %s", test.allocation.spec.Prefix, allocResp.Status.AllocatedPrefix)
				}
			} else {
				if allocResp.Status.AllocatedPrefix == "" || allocResp.Status.Gateway != test.gateway {
					t.Errorf("expected allocation with gateway %s, got allocatedPrefix %s, gateway %s", test.gateway, allocResp.Status.AllocatedPrefix, allocReq.Status.Gateway)
				}
			}
		}
	}

	routes := ipam.GetPrefixes(niCr)
	for _, route := range routes {
		fmt.Println(route)
	}

}
