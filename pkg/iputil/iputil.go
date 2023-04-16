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

type Prefix struct {
	netip.Prefix
}

func NewPrefixInfo(p netip.Prefix) *Prefix {
	return &Prefix{p}
}

func New(prefix string) (*Prefix, error) {
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, err
	}
	return &Prefix{p}, nil
}

func (r *Prefix) GetIPPrefix() netip.Prefix {
	return r.Prefix
}

func (r *Prefix) GetIPAddress() netip.Addr {
	return r.Prefix.Addr()
}

func (r *Prefix) GetIPAddressPrefix() netip.Prefix {
	return netip.PrefixFrom(r.Prefix.Addr(), r.Prefix.Addr().BitLen())
}

func (r *Prefix) GetIPSubnet() netip.Prefix {
	return r.Prefix.Masked()
}

func (r *Prefix) GetSubnetName() string {
	return fmt.Sprintf("%s-%s", r.GetFirstIPAddress().String(), r.GetPrefixLength().String())

}

func (r *Prefix) GetFirstIPAddress() netip.Addr {
	return r.Prefix.Masked().Addr()
}

func (r *Prefix) GetFirstIPPrefix() netip.Prefix {
	return netip.PrefixFrom(r.GetFirstIPAddress(), r.Prefix.Addr().BitLen())
}

func (r *Prefix) GetLastIPAddress() netip.Addr {
	return netipx.PrefixLastIP(r.Prefix)
}

func (r *Prefix) GetLastIPPrefix() netip.Prefix {
	return netip.PrefixFrom(r.GetLastIPAddress(), r.Prefix.Addr().BitLen())
}

func (r *Prefix) GetPrefixLength() PrefixLength {
	return PrefixLength(r.Prefix.Bits())
}

// return 32 or 128
func (r *Prefix) GetAddressPrefixLength() PrefixLength {
	return PrefixLength(r.Prefix.Addr().BitLen())
}

func (r *Prefix) IsFirst() bool {
	return r.GetFirstIPAddress().String() == r.GetIPAddress().String()
}

func (r *Prefix) IsLast() bool {
	return r.GetLastIPAddress().String() == r.GetIPAddress().String()
}

func (r *Prefix) IsNorLastNorFirst() bool {
	return !r.IsFirst() && !r.IsLast()
}

func (r *Prefix) IsIpv4() bool {
	return r.Prefix.Addr().Is4()
}

func (r *Prefix) IsIpv6() bool {
	return r.Prefix.Addr().Is6()
}

func (r *Prefix) GetAddressFamily() AddressFamily {
	if r.IsIpv6() {
		return AddressFamilyIpv6
	}
	return AddressFamilyIpv4
}

func (r *Prefix) IsAddressPrefix() bool {
	return r.GetPrefixLength().String() == "32" ||
		r.GetPrefixLength().String() == "128"
}

func (r *Prefix) GetIPPrefixWithPrefixLength(pl int) netip.Prefix {
	return netip.PrefixFrom(r.GetLastIPAddress(), pl)
}

func (r *Prefix) IsPrefixPresentInSubnetMap(prefix string) bool {
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
