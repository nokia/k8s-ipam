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

package clientproxy

/*
import (
	"context"
	"net"
	"testing"

	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestAllocService_CreateIndex(t *testing.T) {
	l := bufconn.Listen(1024 * 1024)
	t.Cleanup((func() {
		l.Close()
	}))

	s := grpc.NewServer()
	t.Cleanup(func() {
		s.Stop()
	})

	pc := allocpb.UnimplementedAllocationServer{}
	allocpb.RegisterAllocationServer(s, pc)

	go func() {
		if err := s.Serve(l); err != nil {
			t.Errorf("srv.Serve error: %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return l.Dial()
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithContextDialer(dialer))
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.DialContext(context.Background(), "", opts...)
	t.Cleanup(func() {
		conn.Close()
	})
	if err != nil {
		t.Errorf("grpc.DialContext %v", err)
	}

	client := allocpb.NewAllocationClient(conn)
	_, err = client.CreateIndex(context.Background(), &allocpb.AllocRequest{})
	if err != nil {
		t.Errorf("CreateIndex, err: %v", err)
	}

}
*/
