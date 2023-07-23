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
	// system defined inventory
	NephioPositionKey               = "inv.nephio.org/position" // spine, leaf, cluster
	NephioVendorTypeKey             = "inv.nephio.org/vendor-type"
	NephioPlatformKey               = "inv.nephio.org/platform"
	NephioMACAddressKey             = "inv.nephio.org/mac-address"
	NephioSerialNumberKey           = "inv.nephio.org/serial-number"
	NephioGatewayKey                = "inv.nephio.org/gateway"
	NephioLinkTypeKey               = "inv.nephio.org/link-type" // infra, loop
	NephioLink2NodeKey              = "inv.nephio.org/link2node" // 0 or 1
	NephioEndpointGroupKey          = "inv.nephio.org/endpoint-group"
	NephioInventoryNodeNameKey      = "inv.nephio.org/node-name"
	NephioInventoryLinkNameKey      = "inv.nephio.org/link-name"
	NephioInventoryInterfaceNameKey = "inv.nephio.org/interface-name"
	// user defined common
	NephioTopologyKey         = "topo.nephio.org/topology"
	NephioClusterNameKey      = "nephio.org/cluster-name"
	NephioSiteKey             = "nephio.org/site"
	NephioRegionKey           = "nephio.org/region"
	NephioAvailabilityZoneKey = "nephio.org/availability-zone"
	NephioPurposeKey          = "nephio.org/purpose"
	NephioIndexKey            = "nephio.org/index"
	NephioProviderKey         = "nephio.org/provider"
	NephioWiringKey           = "nephio.org/wiring"
	// user defined topology
	NephioInventoryRack                = "inv.nephio.org/rack"
	NephioInventoryPodIndex            = "inv.nephio.org/pod-index"
	NephioInventoryPlaneIndex          = "inv.nephio.org/plane-index"
	NephioInventoryRackIndex           = "inv.nephio.org/rack-index"
	NephioInventoryNodeIndex           = "inv.nephio.org/node-index"
	NephioInventoryNodeRedundancyGroup = "inv.nephio.org/node-redundancy-group"
	NephioInventoryEndpointIndex       = "inv.nephio.org/interface-index"

	RevisionHash = "nephio.org/revision-hash"
)
