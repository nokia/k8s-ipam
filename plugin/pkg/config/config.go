/*
Copyright 2023 Nokia.

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
package config

import (
	"encoding/json"
	"fmt"
)

type Net struct {
	Name       string      `json:"name"`
	CNIVersion string      `json:"name"`
	IPAM       *IPAMConfig `json:"ipam"`
}

// IPAMConfig for k8s-ipam
type IPAMConfig struct {
	NetInstance  string `json:"networkInstance"`
	NetNamespace string `json:"namespace"`
	IPPrefix     string `json:"ipPrefix"`
	Kubeconfig   string `json:"kubeconfig"`
	Name         string
	CNIVersion   string
}

// NewIPAMConfig creates a NetworkConfig from the given network name.
func LoadIPAMConfig(bytes []byte, envArgs string) (*IPAMConfig, error) {
	n := Net{}
	if err := json.Unmarshal(bytes, &n); err != nil {
		return nil, err
	}

	if n.IPAM == nil {
		return nil, fmt.Errorf("IPAM config missing 'ipam' key")
	}

	// Copy net name into IPAM so not to drag Net struct around
	n.IPAM.Name = n.Name
	n.IPAM.CNIVersion = n.CNIVersion

	return n.IPAM, nil
}
