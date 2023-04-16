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

package iputil

import (
	"fmt"
	"net/netip"
	"strconv"

	"go4.org/netipx"
)

type PrefixInfo interface {
	GetSubnetName() string
	GetIPPrefix() netip.Prefix
	GetIPAddress() netip.Addr
	GetIPAddressPrefix() netip.Prefix
	GetIPSubnet() netip.Prefix
	GetFirstIPAddress() netip.Addr
	GetFirstIPPrefix() netip.Prefix
	GetLastIPAddress() netip.Addr
	GetLastIPPrefix() netip.Prefix
	GetPrefixLength() PrefixLength
	GetAddressPrefixLength() PrefixLength
	IsFirst() bool
	IsLast() bool
	IsNorLastNorFirst() bool
	IsIpv4() bool
	IsIpv6() bool
	GetAddressFamily() AddressFamily
	IsAddressPrefix() bool
	GetIPPrefixWithPrefixLength(pl int) netip.Prefix
	IsPrefixPresentInSubnetMap(string) bool
}

type prefixInfo struct {
	p netip.Prefix
}

func NewPrefixInfo(p netip.Prefix) PrefixInfo {
	return &prefixInfo{p: p}
}

func New(prefix string) (PrefixInfo, error) {
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, err
	}
	return &prefixInfo{
		p: p,
	}, nil
}

func (r *prefixInfo) GetIPPrefix() netip.Prefix {
	return r.p
}

func (r *prefixInfo) GetIPAddress() netip.Addr {
	return r.p.Addr()
}

func (r *prefixInfo) GetIPAddressPrefix() netip.Prefix {
	return netip.PrefixFrom(r.p.Addr(), r.p.Addr().BitLen())
}

func (r *prefixInfo) GetIPSubnet() netip.Prefix {
	return r.p.Masked()
}

func (r *prefixInfo) GetSubnetName() string {
	return fmt.Sprintf("%s-%s", r.GetFirstIPAddress().String(), r.GetPrefixLength().String())

}

func (r *prefixInfo) GetFirstIPAddress() netip.Addr {
	return r.p.Masked().Addr()
}

func (r *prefixInfo) GetFirstIPPrefix() netip.Prefix {
	return netip.PrefixFrom(r.GetFirstIPAddress(), r.p.Addr().BitLen())
}

func (r *prefixInfo) GetLastIPAddress() netip.Addr {
	return netipx.PrefixLastIP(r.p)
}

func (r *prefixInfo) GetLastIPPrefix() netip.Prefix {
	return netip.PrefixFrom(r.GetLastIPAddress(), r.p.Addr().BitLen())
}

func (r *prefixInfo) GetPrefixLength() PrefixLength {
	return PrefixLength(r.p.Bits())
}

// return 32 or 128
func (r *prefixInfo) GetAddressPrefixLength() PrefixLength {
	return PrefixLength(r.p.Addr().BitLen())
}

func (r *prefixInfo) IsFirst() bool {
	return r.GetFirstIPAddress().String() == r.GetIPAddress().String()
}

func (r *prefixInfo) IsLast() bool {
	return r.GetLastIPAddress().String() == r.GetIPAddress().String()
}

func (r *prefixInfo) IsNorLastNorFirst() bool {
	return !r.IsFirst() && !r.IsLast()
}

func (r *prefixInfo) IsIpv4() bool {
	return r.p.Addr().Is4()
}

func (r *prefixInfo) IsIpv6() bool {
	return r.p.Addr().Is6()
}

func (r *prefixInfo) GetAddressFamily() AddressFamily {
	if r.IsIpv6() {
		return AddressFamilyIpv6
	}
	return AddressFamilyIpv4
}

func (r *prefixInfo) IsAddressPrefix() bool {
	return r.GetPrefixLength().String() == "32" ||
		r.GetPrefixLength().String() == "128"
}

func (r *prefixInfo) GetIPPrefixWithPrefixLength(pl int) netip.Prefix {
	return netip.PrefixFrom(r.GetLastIPAddress(), pl)
}

func (r *prefixInfo) IsPrefixPresentInSubnetMap(prefix string) bool {
	if r.GetIPSubnet().String() == prefix {
		return true
	}
	if r.GetFirstIPPrefix().String() == prefix {
		return true
	}
	if r.GetLastIPPrefix().String() == prefix {
		return true
	}
	if r.GetIPAddressPrefix().String() == prefix {
		return true
	}
	return false
}

type PrefixLength int

func (r PrefixLength) String() string {
	return strconv.Itoa(int(r))
}

func (r PrefixLength) Int() int {
	return int(r)
}

type AddressFamily string

const (
	AddressFamilyIpv4    AddressFamily = "ipv4"
	AddressFamilyIpv6    AddressFamily = "ipv6"
	AddressFamilyUnknown AddressFamily = "unknown"
)

func (s AddressFamily) String() string {
	switch s {
	case AddressFamilyIpv4:
		return string(AddressFamilyIpv4)
	case AddressFamilyIpv6:
		return string(AddressFamilyIpv6)
	}
	return string(AddressFamilyUnknown)
}
