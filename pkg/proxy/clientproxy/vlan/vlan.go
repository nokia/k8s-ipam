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

package vlan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/meta"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx context.Context, cfg clientproxy.Config) clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation] {
	return clientproxy.New[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation](
		ctx, clientproxy.Config{
			Address:     cfg.Address,
			Name:        "vlan-client-proxy",
			Group:       vlanv1alpha1.GroupVersion.Group, // Group of GVK for event handling
			Normalizefn: NormalizeKRMToAllocPb,
			ValidateFn:  ValidateResponse,
		})
}

// ValidateResponse handes validates changes in the allocation response
// when doing refreshes
func ValidateResponse(origResp *allocpb.AllocResponse, newResp *allocpb.AllocResponse) bool {
	origAlloc := &vlanv1alpha1.VLANAllocation{}
	if err := json.Unmarshal([]byte(origResp.Status), origAlloc); err != nil {
		return false
	}
	newAlloc := &vlanv1alpha1.VLANAllocation{}
	if err := json.Unmarshal([]byte(origResp.Status), newAlloc); err != nil {
		return false
	}
	if origAlloc.Status.VLANID != newAlloc.Status.VLANID ||
		origAlloc.Status.VLANRange != newAlloc.Status.VLANRange {
		return false
	}
	return true

}

// NormalizeKRMToAllocPb normalizes the input to a generalized GRPC allocation request
// First we normalize the object to an allocation -> this is specific to the source/own client.Object
// Once normalized we can do generic processing -> add system desfined labels in the user defined labels
// in the spec and transform to an allocPB proto message
func NormalizeKRMToAllocPb(o client.Object, d any) (*allocpb.AllocRequest, error) {
	var alloc *vlanv1alpha1.VLANAllocation
	expiryTime := "never"
	nsnName := o.GetName()
	switch o.GetObjectKind().GroupVersionKind().Kind {
	case vlanv1alpha1.VLANKind:
		cr, ok := o.(*vlanv1alpha1.VLAN)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to IPPrefix failed")
		}
		// create a new allocation CR from the owner CR
		objectMeta := cr.ObjectMeta
		if len(objectMeta.GetLabels()) == 0 {
			objectMeta.Labels = map[string]string{}
		}
		objectMeta.Labels[allocv1alpha1.NephioOwnerGvkKey] = meta.GVKToString(vlanv1alpha1.VLANGroupVersionKind)
		alloc = vlanv1alpha1.BuildVLANAllocation(
			objectMeta,
			vlanv1alpha1.VLANAllocationSpec{
				VLANDatabase: cr.Spec.VLANDatabase,
				VLANID:       cr.Spec.VLANID,
				VLANRange:    cr.Spec.VLANRange,
			},
			vlanv1alpha1.VLANAllocationStatus{})
	case vlanv1alpha1.VLANAllocationKind:
		cr, ok := o.(*vlanv1alpha1.VLANAllocation)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to IPAllocation failed")
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
	default:
		return nil, fmt.Errorf("cannot allocate resource for unknown kind, got %s", o.GetObjectKind().GroupVersionKind().Kind)
	}

	// generic processing
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
			vlanv1alpha1.VLANAllocationGroupVersionKind),
		nil
}
