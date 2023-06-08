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

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx context.Context, cfg clientproxy.Config) clientproxy.Proxy[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim] {
	return clientproxy.New[*vlanv1alpha1.VLANIndex, *vlanv1alpha1.VLANClaim](
		ctx, clientproxy.Config{
			Address:     cfg.Address,
			Name:        "vlan-client-proxy",
			Group:       vlanv1alpha1.GroupVersion.Group, // Group of GVK for event handling
			Normalizefn: NormalizeKRMToResourcePb,
			ValidateFn:  ValidateResponse,
		})
}

// ValidateResponse handes validates changes in the claim response
// when doing refreshes
func ValidateResponse(origResp *resourcepb.ClaimResponse, newResp *resourcepb.ClaimResponse) bool {
	origClaim := vlanv1alpha1.VLANClaim{}
	if err := json.Unmarshal([]byte(origResp.Status), &origClaim); err != nil {
		return false
	}
	newClaim := vlanv1alpha1.VLANClaim{}
	if err := json.Unmarshal([]byte(origResp.Status), &newClaim); err != nil {
		return false
	}
	if origClaim.Status.VLANID != nil {
		if newClaim.Status.VLANID == nil {
			return false
		}
		if *origClaim.Status.VLANID != *newClaim.Status.VLANID {
			return false
		}
	}
	if origClaim.Status.VLANRange != nil {
		if newClaim.Status.VLANRange == nil {
			return false
		}
		if *origClaim.Status.VLANRange != *newClaim.Status.VLANRange {
			return false
		}
	}
	return true

}

// NormalizeKRMToResourcePb normalizes the input to a generalized GRPC claim request
// First we normalize the object to an claim -> this is specific to the source/own client.Object
// Once normalized we can do generic processing -> add system desfined labels in the user defined labels
// in the spec and transform to an resourcePB proto message
func NormalizeKRMToResourcePb(o client.Object, d any) (*resourcepb.ClaimRequest, error) {
	var claim *vlanv1alpha1.VLANClaim
	expiryTime := "never"
	nsnName := o.GetName()
	switch o.GetObjectKind().GroupVersionKind().Kind {
	case vlanv1alpha1.VLANKind:
		cr, ok := o.(*vlanv1alpha1.VLAN)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to IPPrefix failed")
		}
		// create a new claimation CR from the owner CR
		objectMeta := cr.ObjectMeta
		if len(objectMeta.GetLabels()) == 0 {
			objectMeta.Labels = map[string]string{}
		}
		objectMeta.Labels[resourcev1alpha1.NephioOwnerGvkKey] = meta.GVKToString(vlanv1alpha1.VLANGroupVersionKind)
		claim = vlanv1alpha1.BuildVLANClaim(
			objectMeta,
			vlanv1alpha1.VLANClaimSpec{
				VLANIndex: cr.Spec.VLANIndex,
				VLANID:    cr.Spec.VLANID,
				VLANRange: cr.Spec.VLANRange,
			},
			vlanv1alpha1.VLANClaimStatus{})
	case vlanv1alpha1.VLANClaimKind:
		cr, ok := o.(*vlanv1alpha1.VLANClaim)
		if !ok {
			return nil, fmt.Errorf("unexpected error casting object to VLANClaim  failed")
		}
		// given the cr exists we just do a deepcopy
		claim = cr.DeepCopy()
		// addExpiryTime
		t := time.Now().Add(time.Minute * 60)
		b, err := t.MarshalText()
		if err != nil {
			return nil, err
		}
		expiryTime = string(b)
	default:
		return nil, fmt.Errorf("cannot claimate resource for unknown kind, got %s", o.GetObjectKind().GroupVersionKind().Kind)
	}

	// generic processing
	// add system defined labels to the user defined label section of the claimation spec
	claim.AddOwnerLabelsToCR()
	// marshal the claim
	b, err := json.Marshal(claim)
	if err != nil {
		return nil, err
	}
	return clientproxy.BuildResourcePb(
			o,
			nsnName,
			string(b),
			expiryTime,
			vlanv1alpha1.VLANClaimGroupVersionKind),
		nil
}
