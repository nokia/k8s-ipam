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
	"strconv"
	"testing"
)

func TestPrefixInfo(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   []string
	}{
		{
			name:   "first address in prefix",
			prefix: "10.0.0.0/24",
			want: []string{
				"10.0.0.0-24",
				"10.0.0.0/24",
				"10.0.0.0",
				"10.0.0.0/32",
				"10.0.0.0/24",
				"10.0.0.0",
				"10.0.0.0/32",
				"10.0.0.255",
				"10.0.0.255/32",
				"24",
				"32",
				"true",
				"false",
				"false",
				"true",
				"false",
				"ipv4",
				"false",
			},
		},
		{
			name:   "regular address in prefix",
			prefix: "10.0.0.1/24",
			want: []string{
				"10.0.0.0-24",
				"10.0.0.1/24",
				"10.0.0.1",
				"10.0.0.1/32",
				"10.0.0.0/24",
				"10.0.0.0",
				"10.0.0.0/32",
				"10.0.0.255",
				"10.0.0.255/32",
				"24",
				"32",
				"false",
				"false",
				"true",
				"true",
				"false",
				"ipv4",
				"false",
			},
		},
		{
			name:   "last address in prefix",
			prefix: "10.0.0.255/24",
			want: []string{
				"10.0.0.0-24",
				"10.0.0.255/24",
				"10.0.0.255",
				"10.0.0.255/32",
				"10.0.0.0/24",
				"10.0.0.0",
				"10.0.0.0/32",
				"10.0.0.255",
				"10.0.0.255/32",
				"24",
				"32",
				"false",
				"true",
				"false",
				"true",
				"false",
				"ipv4",
				"false",
			},
		},
		{
			name:   "ipv6 address in prefix",
			prefix: "2000::1/64",
			want: []string{
				"2000---64",
				"2000::1/64",
				"2000::1",
				"2000::1/128",
				"2000::/64",
				"2000::",
				"2000::/128",
				"2000::ffff:ffff:ffff:ffff",
				"2000::ffff:ffff:ffff:ffff/128",
				"64",
				"128",
				"false",
				"false",
				"true",
				"false",
				"true",
				"ipv6",
				"false",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := New(tt.prefix)

			if got := p.GetSubnetName(); got != tt.want[0] {
				t.Errorf("GetSubnetName() = %s, want %s", got, tt.want[0])
			}
			if got := p.GetIPPrefix().String(); got != tt.want[1] {
				t.Errorf("GetIPPrefix() = %s, want %s", got, tt.want[1])
			}
			if got := p.GetIPAddress().String(); got != tt.want[2] {
				t.Errorf("GetIPAddress() = %s, want %s", got, tt.want[2])
			}
			if got := p.GetIPAddressPrefix().String(); got != tt.want[3] {
				t.Errorf("GetIPAddressPrefix() = %s, want %s", got, tt.want[3])
			}
			if got := p.GetIPSubnet().String(); got != tt.want[4] {
				t.Errorf("GetIPSubnet() = %s, want %s", got, tt.want[4])
			}
			if got := p.GetFirstIPAddress().String(); got != tt.want[5] {
				t.Errorf("GetFirstIPAddress() = %s, want %s", got, tt.want[5])
			}
			if got := p.GetFirstIPPrefix().String(); got != tt.want[6] {
				t.Errorf("GetFirstIPPrefix() = %s, want %s", got, tt.want[6])
			}
			if got := p.GetLastIPAddress().String(); got != tt.want[7] {
				t.Errorf("GetLastIPAddress() = %s, want %s", got, tt.want[7])
			}
			if got := p.GetLastIPPrefix().String(); got != tt.want[8] {
				t.Errorf("GetLastIPPrefix() = %s, want %s", got, tt.want[8])
			}
			if got := p.GetPrefixLength().String(); got != tt.want[9] {
				t.Errorf("GetPrefixLength() = %s, want %s", got, tt.want[9])
			}
			if got := p.GetAddressPrefixLength().String(); got != tt.want[10] {
				t.Errorf("GetAddressPrefixLength() = %s, want %s", got, tt.want[10])
			}
			if got := strconv.FormatBool(p.IsFirst()); got != tt.want[11] {
				t.Errorf("IsFirst() = %s, want %s", got, tt.want[11])
			}
			if got := strconv.FormatBool(p.IsLast()); got != tt.want[12] {
				t.Errorf("IsLast() = %s, want %s", got, tt.want[12])
			}
			if got := strconv.FormatBool(p.IsNorLastNorFirst()); got != tt.want[13] {
				t.Errorf("IsNorLastNorFirst() = %s, want %s", got, tt.want[13])
			}
			if got := strconv.FormatBool(p.IsIpv4()); got != tt.want[14] {
				t.Errorf("IsIpv4() = %s, want %s", got, tt.want[14])
			}
			if got := strconv.FormatBool(p.IsIpv6()); got != tt.want[15] {
				t.Errorf("IsIpv6() = %s, want %s", got, tt.want[15])
			}
			if got := p.GetAddressFamily().String(); got != tt.want[16] {
				t.Errorf("GetAddressFamily() = %s, want %s", got, tt.want[16])
			}
			if got := strconv.FormatBool(p.IsAddressPrefix()); got != tt.want[17] {
				t.Errorf("IsAddressPrefix() = %s, want %s", got, tt.want[17])
			}
		})
	}
}
