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

package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/internal/utils/util"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx context.Context, cfg clientproxy.Config) clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation] {
	return clientproxy.New[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation](
		ctx, clientproxy.Config{
			Address:     cfg.Address,
			Name:        "ipam-client-proxy",
			Group:       ipamv1alpha1.GroupVersion.Group, // Group of GVK for event handling
			Normalizefn: NormalizeKRMToAllocPb,
			ValidateFn:  ValidateResponse,
		})
}

// ValidateResponse handes validates changes in the allocation response
// when doing refreshes
func ValidateResponse(origResp *allocpb.AllocResponse, newResp *allocpb.AllocResponse) bool {
	origAlloc := &ipamv1alpha1.IPAllocation{}
	if err := json.Unmarshal([]byte(origResp.Status), origAlloc); err != nil {
		return false
	}
	newAlloc := &ipamv1alpha1.IPAllocation{}
	if err := json.Unmarshal([]byte(origResp.Status), newAlloc); err != nil {
		return false
	}
	if origAlloc.Status.Prefix != newAlloc.Status.Prefix ||
		origAlloc.Status.Gateway != newAlloc.Status.Gateway {
		return false
	}
	return true
}

// NormalizeKRMToAllocPb normalizes the input to a generalized GRPC allocation request
// First we normalize the object to an allocation -> this is specific to the source/own client.Object
// Once normalized we can do generic processing -> add system desfined labels in the user defined labels
// in the spec and transform to an allocPB proto message
func NormalizeKRMToAllocPb(o client.Object, d any) (*allocpb.AllocRequest, error) {
	var alloc *ipamv1alpha1.IPAllocation
	expiryTime := "never"
	nsnName := o.GetName()
	switch o.GetObjectKind().GroupVersionKind().Kind {
	case ipamv1alpha1.IPPrefixKind:
		cr, ok := o.(*ipamv1alpha1.IPPrefix)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object expected: %s, got: %v", o.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(o))
		}
		// create a new allocation CR from the owner CR
		pi, err := iputil.New(cr.Spec.Prefix)
		if err != nil {
			return nil, err
		}
		objectMeta := cr.ObjectMeta
		if len(objectMeta.GetLabels()) == 0 {
			objectMeta.Labels = map[string]string{}
		}
		objectMeta.Labels[allocv1alpha1.NephioOwnerGvkKey] = meta.GVKToString(ipamv1alpha1.IPPrefixGroupVersionKind)
		alloc = ipamv1alpha1.BuildIPAllocation(
			objectMeta,
			ipamv1alpha1.IPAllocationSpec{
				Kind:            cr.Spec.Kind,
				NetworkInstance: cr.Spec.NetworkInstance,
				Prefix:          &cr.Spec.Prefix,
				PrefixLength:    util.PointerUint8(pi.GetPrefixLength().Int()),
				CreatePrefix:    pointer.Bool(true),
				AllocationLabels: allocv1alpha1.AllocationLabels{
					UserDefinedLabels: cr.Spec.UserDefinedLabels,
				},
			},
			ipamv1alpha1.IPAllocationStatus{})
	case ipamv1alpha1.IPAllocationKind:
		cr, ok := o.(*ipamv1alpha1.IPAllocation)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object expected: %s, got: %v", o.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(o))
		}
		// given the cr exists we just do a deepcopy
		alloc = cr.DeepCopy()
		// addExpiryTime
		t := time.Now().Add(time.Minute * 60)
		b, err := t.MarshalText()
		if err != nil {
			return nil, err
		}
		expiryTime = string(b)
	case ipamv1alpha1.NetworkInstanceKind:
		cr, ok := o.(*ipamv1alpha1.NetworkInstance)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object expected: %s, got: %v", o.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(o))
		}
		prefix, ok := d.(ipamv1alpha1.Prefix)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to Ip Prefix failed")
		}
		// create a new allocation CR from the owner CR
		objectMeta := cr.ObjectMeta
		// the name of the nsn is set to the prefixName within the network instance
		objectMeta.Name = cr.GetNameFromNetworkInstancePrefix(prefix.Prefix)
		nsnName = cr.GetNameFromNetworkInstancePrefix(prefix.Prefix)
		if len(objectMeta.GetLabels()) == 0 {
			objectMeta.Labels = map[string]string{}
		}
		objectMeta.Labels[allocv1alpha1.NephioOwnerGvkKey] = meta.GVKToString(ipamv1alpha1.NetworkInstanceGroupVersionKind)

		pi, err := iputil.New(prefix.Prefix)
		if err != nil {
			return nil, err
		}
		alloc = ipamv1alpha1.BuildIPAllocation(
			objectMeta,
			ipamv1alpha1.IPAllocationSpec{
				Kind: ipamv1alpha1.PrefixKindAggregate,
				NetworkInstance: corev1.ObjectReference{
					Name:      cr.GetName(),
					Namespace: cr.GetNamespace(),
				},
				Prefix:       &prefix.Prefix,
				PrefixLength: util.PointerUint8(pi.GetPrefixLength().Int()),
				CreatePrefix: pointer.Bool(true),
				AllocationLabels: allocv1alpha1.AllocationLabels{
					UserDefinedLabels: prefix.UserDefinedLabels,
				},
			},
			ipamv1alpha1.IPAllocationStatus{})
	default:
		return nil, fmt.Errorf("cannot allocate prefix for unknown kind, got %s", o.GetObjectKind().GroupVersionKind().Kind)
	}

	// do generic processing
	// add system defined labels to the user defined label section of the allocation spec
	alloc.AddOwnerLabelsToCR()
	// marshal the alloc
	b, err := json.Marshal(alloc)
	if err != nil {
		return nil, err
	}
	return clientproxy.BuildAllocPb(
			o,
			nsnName,
			string(b),
			expiryTime,
			ipamv1alpha1.IPAllocationGroupVersionKind),
		nil
}
