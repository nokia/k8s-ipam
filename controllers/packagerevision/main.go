package main

import (
	"context"
	"fmt"
	"os"

	client "github.com/henderiw-nephio/ipam/pkg/alloc/allocclient"
	"github.com/henderiw-nephio/ipam/pkg/alloc/allocpb"
)

func main() {
	ctx := context.Background()
	c, err := client.New(ctx, &client.Config{
		Address:  "127.0.0.1:9999",
		Insecure: true,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	resp, err := c.AllocationRequest(ctx, &allocpb.Request{
		Namespace: "default",
		Name:      "grpcAlloc1",
		Kind:      "ipam",
		Labels: map[string]string{
			"grpc-client": "test",
		},
		Spec: &allocpb.Spec{
			Selector: map[string]string{
				"nephio.org/network-instance": "network-1",
				"nephio.org/prefix-name":      "network1-prefix1",
			},
		},
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("response: prefix %s, parentprefix: %s", resp.GetPrefix(), resp.GetParentPrefix())

}
