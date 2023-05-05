package meta

import (
	"strings"

	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"k8s.io/apimachinery/pkg/types"
)

func AllocPbNsnToString(nsn *allocpb.NSN) string {
	return types.NamespacedName{Namespace: nsn.Namespace, Name: nsn.Name}.String()
}

func StringToAllocPbNsn(s string) *allocpb.NSN {
	split := strings.Split(s, "/")
	if len(split) > 1 {
		return &allocpb.NSN{
			Namespace: split[0],
			Name:      split[1],
		}
	}
	return &allocpb.NSN{
		Namespace: "default",
		Name:      s,
	}
}

func GetAllocPbGVKFromTypeNSN(nsn types.NamespacedName) *allocpb.NSN {
	return &allocpb.NSN{
		Namespace: nsn.Namespace,
		Name:      nsn.Name,
	}
}

func GetTypeNSNFromAllocPbNSN(nsn *allocpb.NSN) types.NamespacedName {
	return types.NamespacedName{
		Namespace: nsn.Namespace,
		Name:      nsn.Name,
	}
}
