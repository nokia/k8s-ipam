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
	NephioPositionKey      = "inv.nephio.org/position" // spine, leaf, cluster
	NephioVendorTypeKey    = "inv.nephio.org/vendor-type"
	NephioPlatformKey      = "inv.nephio.org/platform"
	NephioMACAddressKey    = "nephio.org/mac-address"
	NephioSerialNumberKey  = "nephio.org/serial-number"
	NephioGatewayKey       = "nephio.org/gateway"
	NephioLinkTypeKey      = "nephio.org/link-type" // infra, loop
	NephioLink2NodeKey     = "nephio.org/link2node" // 0 or 1
	NephioEndpointGroupKey = "nephio.org/endpoint-group"
	NephioNodeNameKey      = "nephio.org/node-name"
	NephioLinkNameKey      = "nephio.org/link-name"
	NephioInterfaceNameKey = "nephio.org/interface-name"
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
	NephioTopologyPosition   = "topo.nephio.org/position"
	NephioTopologyRack       = "topo.nephio.org/rack"
	NephioTopologyPodIndex   = "topo.nephio.org/pod-index"
	NephioTopologyPlaneIndex = "topo.nephio.org/plane-index"
	NephioTopologyRackIndex  = "topo.nephio.org/rack-index"
	NephioTopologyNodeIndex  = "topo.nephio.org/node-index"

	RevisionHash = "nephio.org/revision-hash"
)
