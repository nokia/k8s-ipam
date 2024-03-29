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

package v1alpha1

const (
	// app
	NephioApp = "nephio.org/app"
	// action
	NephioAPIAction = "nephio.org/action" // values get
	// system defined common
	NephioOwnerGvkKey          = "nephio.org/owner-gvk"
	NephioOwnerNsnNameKey      = "nephio.org/owner-nsn-name"
	NephioOwnerNsnNamespaceKey = "nephio.org/owner-nsn-namespace"
	NephioGvkKey               = "nephio.org/gvk"
	NephioNsnNameKey           = "nephio.org/nsn-name"
	NephioNsnNamespaceKey      = "nephio.org/nsn-namespace"
	NephioOwnerRefKey          = "nephio.org/owner-ref"
	// system defined ipam
	NephioPrefixKindKey    = "nephio.org/prefix-kind"
	NephioAddressFamilyKey = "nephio.org/address-family"
	NephioSubnetKey        = "nephio.org/subnet" // this is the subnet in prefix annotation used for GW selection
	NephioPoolKey          = "nephio.org/pool"
	NephioGatewayKey       = "nephio.org/gateway"
	// user defined common
	NephioClusterNameKey       = "nephio.org/cluster-name"
	NephioSiteNameKey          = "nephio.org/site-name"
	NephioRegionKey            = "nephio.org/region"
	NephioAvailabilityZoneKey  = "nephio.org/availability-zone"
	NephioInterfaceKey         = "nephio.org/interface"
	NephioNetworkNameKey       = "nephio.org/network-name"
	NephioPurposeKey           = "nephio.org/purpose"
	NephioApplicationPartOfKey = "app.kubernetes.io/part-of"
	NephioIndexKey             = "nephio.org/index"
	// status ipam
	NephioClaimedPrefix  = "nephio.org/claimed-prefix"
	NephioClaimedGateway = "nephio.org/claimed-gateway"
)
