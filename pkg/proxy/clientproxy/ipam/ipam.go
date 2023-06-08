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

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx context.Context, cfg clientproxy.Config) clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim] {
	return clientproxy.New[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPClaim](
		ctx, clientproxy.Config{
			Address:     cfg.Address,
			Name:        "ipam-client-proxy",
			Group:       ipamv1alpha1.GroupVersion.Group, // Group of GVK for event handling
			Normalizefn: NormalizeKRMToResourcePb,
			ValidateFn:  ValidateResponse,
		})
}

// ValidateResponse handes validates changes in the claim response
// when doing refreshes
func ValidateResponse(origResp *resourcepb.ClaimResponse, newResp *resourcepb.ClaimResponse) bool {
	origClaim := ipamv1alpha1.IPClaim{}
	if err := json.Unmarshal([]byte(origResp.Status), &origClaim); err != nil {
		return false
	}
	newClaim := ipamv1alpha1.IPClaim{}
	if err := json.Unmarshal([]byte(origResp.Status), &newClaim); err != nil {
		return false
	}
	if origClaim.Status.Prefix != nil {
		if newClaim.Status.Prefix == nil {
			return false
		}
		if *origClaim.Status.Prefix != *newClaim.Status.Prefix {
			return false
		}
	}
	if origClaim.Status.Gateway != nil {
		if newClaim.Status.Gateway == nil {
			return false
		}
		if *origClaim.Status.Gateway != *newClaim.Status.Gateway {
			return false
		}
	}
	return true
}

// NormalizeKRMToResourcePb normalizes the input to a generalized GRPC resource request
// First we normalize the object to an claim -> this is specific to the source/own client.Object
// Once normalized we can do generic processing -> add system desfined labels in the user defined labels
// in the spec and transform to an resourcePB proto message
func NormalizeKRMToResourcePb(o client.Object, d any) (*resourcepb.ClaimRequest, error) {
	b, expiryTime, nsnName, err := NormalizeKRM(o, d)
	if err != nil {
		return nil, err
	}
	return clientproxy.BuildResourcePb(
			o,
			nsnName,
			string(b),
			expiryTime,
			ipamv1alpha1.IPClaimGroupVersionKind),
		nil
}

func NormalizeKRMToBytes(o client.Object, d any) ([]byte, error) {
	b, _, _, err := NormalizeKRM(o, d)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// NormalizeKRMToResourcePb normalizes the input to a generalized GRPC resource request
// First we normalize the object to an claim -> this is specific to the source/own client.Object
// Once normalized we can do generic processing -> add system desfined labels in the user defined labels
// in the spec and transform to an resourcePB proto message
func NormalizeKRM(o client.Object, d any) ([]byte, string, string, error) {
	var claim *ipamv1alpha1.IPClaim
	expiryTime := "never"
	nsnName := o.GetName()
	switch o.GetObjectKind().GroupVersionKind().Kind {
	case ipamv1alpha1.IPPrefixKind:
		cr, ok := o.(*ipamv1alpha1.IPPrefix)
		if !ok {
			return nil, "", "", fmt.Errorf("unexpected error casting object expected: %s, got: %v", o.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(o))
		}
		// create a new claim CR from the owner CR
		pi, err := iputil.New(cr.Spec.Prefix)
		if err != nil {
			return nil, "", "", err
		}
		objectMeta := cr.ObjectMeta
		if len(objectMeta.GetLabels()) == 0 {
			objectMeta.Labels = map[string]string{}
		}
		objectMeta.Labels[resourcev1alpha1.NephioOwnerGvkKey] = meta.GVKToString(ipamv1alpha1.IPPrefixGroupVersionKind)
		claim = ipamv1alpha1.BuildIPClaim(
			objectMeta,
			ipamv1alpha1.IPClaimSpec{
				Kind:            cr.Spec.Kind,
				NetworkInstance: cr.Spec.NetworkInstance,
				Prefix:          &cr.Spec.Prefix,
				PrefixLength:    util.PointerUint8(pi.GetPrefixLength().Int()),
				CreatePrefix:    pointer.Bool(true),
				ClaimLabels: resourcev1alpha1.ClaimLabels{
					UserDefinedLabels: cr.Spec.UserDefinedLabels,
				},
			},
			ipamv1alpha1.IPClaimStatus{})
	case ipamv1alpha1.IPClaimKind:
		cr, ok := o.(*ipamv1alpha1.IPClaim)
		if !ok {
			return nil, "", "", fmt.Errorf("unexpected error casting object expected: %s, got: %v", o.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(o))
		}
		// given the cr exists we just do a deepcopy
		claim = cr.DeepCopy()
		// addExpiryTime
		t := time.Now().Add(time.Minute * 60)
		b, err := t.MarshalText()
		if err != nil {
			return nil, "", "", err
		}
		expiryTime = string(b)
	case ipamv1alpha1.NetworkInstanceKind:
		cr, ok := o.(*ipamv1alpha1.NetworkInstance)
		if !ok {
			return nil, "", "", fmt.Errorf("unexpected error casting object expected: %s, got: %v", o.GetObjectKind().GroupVersionKind().Kind, reflect.TypeOf(o))
		}
		prefix, ok := d.(ipamv1alpha1.Prefix)
		if !ok {
			return nil, "", "", fmt.Errorf("unexpected error casting object to Ip Prefix failed")
		}
		// create a new claim CR from the owner CR
		objectMeta := cr.ObjectMeta
		// the name of the nsn is set to the prefixName within the network instance
		objectMeta.Name = cr.GetNameFromNetworkInstancePrefix(prefix.Prefix)
		nsnName = cr.GetNameFromNetworkInstancePrefix(prefix.Prefix)
		if len(objectMeta.GetLabels()) == 0 {
			objectMeta.Labels = map[string]string{}
		}
		objectMeta.Labels[resourcev1alpha1.NephioOwnerGvkKey] = meta.GVKToString(ipamv1alpha1.NetworkInstanceGroupVersionKind)

		pi, err := iputil.New(prefix.Prefix)
		if err != nil {
			return nil, "", "", err
		}
		claim = ipamv1alpha1.BuildIPClaim(
			objectMeta,
			ipamv1alpha1.IPClaimSpec{
				Kind: ipamv1alpha1.PrefixKindAggregate,
				NetworkInstance: corev1.ObjectReference{
					Name:      cr.GetName(),
					Namespace: cr.GetNamespace(),
				},
				Prefix:       &prefix.Prefix,
				PrefixLength: util.PointerUint8(pi.GetPrefixLength().Int()),
				CreatePrefix: pointer.Bool(true),
				ClaimLabels: resourcev1alpha1.ClaimLabels{
					UserDefinedLabels: prefix.UserDefinedLabels,
				},
			},
			ipamv1alpha1.IPClaimStatus{})
	default:
		return nil, "", "", fmt.Errorf("cannot claim prefix for unknown kind, got %s", o.GetObjectKind().GroupVersionKind().Kind)
	}

	// do generic processing
	// add system defined labels to the user defined label section of the claim spec
	claim.AddOwnerLabelsToCR()
	// marshal the claim
	b, err := json.Marshal(claim)
	if err != nil {
		return nil, "", "", err
	}
	return b, expiryTime, nsnName, nil
}
