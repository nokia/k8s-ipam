package main

import (
	"os"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/nokia/k8s-ipam/pkg/ipam-fn/function"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/ipam"
)

func main() {
	r := &function.FnR{
		IpamClientProxy: ipam.NewMock(),
	}

	if err := fn.AsMain(fn.ResourceListProcessorFunc(r.Run)); err != nil {
		os.Exit(1)
	}
}
