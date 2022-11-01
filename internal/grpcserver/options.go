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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
)

func (s *GrpcServer) serverOpts(ctx context.Context) ([]grpc.ServerOption, error) {
	if s.config.Insecure {
		return []grpc.ServerOption{
			grpc.Creds(insecure.NewCredentials()),
		}, nil
	}

	tlsConfig, err := s.createTLSConfig(ctx)
	if err != nil {
		return nil, err
	}
	return []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	}, nil

}

func (s *GrpcServer) createTLSConfig(ctx context.Context) (*tls.Config, error) {

	caPath := filepath.Join(s.config.CertDir, s.config.CaName)

	ca, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read client CA cert: %w", err)
	}

	certPath := filepath.Join(s.config.CertDir, s.config.CertName)
	keyPath := filepath.Join(s.config.CertDir, s.config.KeyName)

	certWatcher, err := certwatcher.New(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := certWatcher.Start(ctx); err != nil {
			s.l.Info("certificate watcher", "error", err)
		}
	}()

	tlsConfig := &tls.Config{
		GetCertificate: certWatcher.GetCertificate,
	}
	if len(ca) != 0 {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(ca)
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}
