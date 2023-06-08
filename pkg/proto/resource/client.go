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

package resource

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultTimeout = 30 * time.Second
	maxMsgSize     = 512 * 1024 * 1024
)

type Client interface {
	Delete() error
	Get() resourcepb.ResourceClient
}

func New(cfg *Config) (Client, error) {
	c := &client{
		cfg: cfg,
	}
	return c, c.create()
}

type client struct {
	cfg              *Config
	conn             *grpc.ClientConn
	resourcePbClient resourcepb.ResourceClient
}

func (r *client) create() error {
	if r.cfg == nil {
		return fmt.Errorf("must provide non-nil Configw")
	}
	var opts []grpc.DialOption
	fmt.Printf("grpc client config: %v\n", r.cfg)
	if r.cfg.Insecure {
		//opts = append(opts, grpc.WithInsecure())
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig, err := r.newTLS()
		if err != nil {
			return err
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}
	timeoutCtx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var err error
	r.conn, err = grpc.DialContext(timeoutCtx, r.cfg.Address, opts...)
	if err != nil {
		return err
	}
	//defer conn.Close()
	r.resourcePbClient = resourcepb.NewResourceClient(r.conn)

	return nil
}

func (r *client) Get() resourcepb.ResourceClient {
	return r.resourcePbClient
}

func (r *client) Delete() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *client) newTLS() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		Renegotiation:      tls.RenegotiateNever,
		InsecureSkipVerify: r.cfg.SkipVerify,
	}
	//err := loadCerts(tlsConfig)
	//if err != nil {
	//	return nil, err
	//}
	return tlsConfig, nil
}

/*
func loadCerts(tlscfg *tls.Config) error {
	if c.TLSCert != "" && c.TLSKey != "" {
		certificate, err := tls.LoadX509KeyPair(*c.TLSCert, *c.TLSKey)
		if err != nil {
			return err
		}
		tlscfg.Certificates = []tls.Certificate{certificate}
		tlscfg.BuildNameToCertificate()
	}
	if c.TLSCA != nil && *c.TLSCA != "" {
		certPool := x509.NewCertPool()
		caFile, err := ioutil.ReadFile(*c.TLSCA)
		if err != nil {
			return err
		}
		if ok := certPool.AppendCertsFromPEM(caFile); !ok {
			return errors.New("failed to append certificate")
		}
		tlscfg.RootCAs = certPool
	}
	return nil
}
*/
