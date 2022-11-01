/*
Copyright 2022 The Nephio Authors.

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

type AddressFamily string

const (
	AddressFamilyIpv4 AddressFamily = "ipv4"
	AddressFamilyIpv6 AddressFamily = "ipv6"
)

func (s AddressFamily) String() string {
	switch s {
	case AddressFamilyIpv4:
		return "ipv4"
	case AddressFamilyIpv6:
		return "ipv6"
	}
	return "unknown"
}

type Origin string

const (
	OriginIPPrefix     Origin = "prefix"
	OriginIPAllocation Origin = "allocation"
)
