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

const (
	// ipam system defined
	NephioNetworkInstanceKey    = "nephio.org/network-instance"
	NephioPrefixKindKey         = "nephio.org/prefix-kind"
	NephioPrefixKey             = "nephio.org/prefix"
	NephioPrefixLengthKey       = "nephio.org/prefix-length"
	NephioAddressFamilyKey      = "nephio.org/address-family"
	//NephioIPPrefixNameKey       = "nephio.org/prefix-name"
	NephioSubnetKey             = "nephio.org/subnet" // this is the subnet in prefix annotation
	NephioParentPrefixLengthKey = "nephio.org/parent-prefix-length"
	NephioIPAllocactionNameKey  = "nephio.org/allocation-name"
	NephioPoolKey               = "nephio.org/pool"
	NephioGatewayKey            = "nephio.org/gateway"
	NephioOwnerGvkKey           = "nephio.org/owner-gvk"
	NephioOwnerNsnKey           = "nephio.org/owner-nsn"
	NephioGvkKey                = "nephio.org/gvk"
	NephioNsnKey                = "nephio.org/nsn"
	//NephioSubnetNameKey         = "nephio.org/subnet-name" // this is the subnet string or name given in the spec/selector
	//ipam user define
	NephioInterfaceKey         = "nephio.org/interface"
	NephioNetworkNameKey       = "nephio.org/network-name"
	NephioPurposeKey           = "nephio.org/purpose"
	NephioApplicationPartOfKey = "app.kubernetes.io/part-of"
	NephioIndexKey             = "nephio.org/index"
	// ipam status
	NephioAllocatedPrefix  = "nephio.org/allocated-prefix"
	NephioAllocatedGateway = "nephio.org/allocated-gateway"
)
