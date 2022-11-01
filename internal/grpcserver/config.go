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

package grpcserver

import (
	"time"
)

const (
	defaultAddress = ":9999"
	defaultMaxRPC  = 600
	defaultTimeout = time.Minute
)

type Config struct {
	// gRPC server address
	Address string

	// insecure server
	Insecure bool

	// MaxRPC
	MaxRPC int64

	// request timeout
	Timeout time.Duration

	// CertDir is the directory that contains the server key and certificate. The
	// server key and certificate.
	CertDir string

	// CertName is the server certificate name. Defaults to tls.crt.
	CertName string

	// KeyName is the server key name. Defaults to tls.key.
	KeyName string

	// CaName is the ca certificate name. Defaults to ca.crt.
	CaName string
}

func (c *Config) setDefaults() {
	if c.Address == "" {
		c.Address = ":" + defaultAddress
	}
	if c.MaxRPC <= 0 {
		c.MaxRPC = defaultMaxRPC
	}

	/*
		if len(c.CertDir) == 0 {
			c.CertDir = filepath.Join(os.TempDir(), "k8s-grpc-server", "serving-certs")
		}

		if len(c.CertName) == 0 {
			c.CertName = "tls.crt"
		}

		if len(c.KeyName) == 0 {
			c.KeyName = "tls.key"
		}
		if len(c.CaName) == 0 {
			c.CaName = "ca.crt"
		}
	*/
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
}
