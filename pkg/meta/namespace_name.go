package meta

import (
	"strings"

	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"k8s.io/apimachinery/pkg/types"
)

func ResourcePbNsnToString(nsn *resourcepb.NSN) string {
	return types.NamespacedName{Namespace: nsn.Namespace, Name: nsn.Name}.String()
}

func StringToResourcePbNsn(s string) *resourcepb.NSN {
	split := strings.Split(s, "/")
	if len(split) > 1 {
		return &resourcepb.NSN{
			Namespace: split[0],
			Name:      split[1],
		}
	}
	return &resourcepb.NSN{
		Namespace: "default",
		Name:      s,
	}
}

func GetResourcePbGVKFromTypeNSN(nsn types.NamespacedName) *resourcepb.NSN {
	return &resourcepb.NSN{
		Namespace: nsn.Namespace,
		Name:      nsn.Name,
	}
}

func GetTypeNSNFromResourcePbNSN(nsn *resourcepb.NSN) types.NamespacedName {
	return types.NamespacedName{
		Namespace: nsn.Namespace,
		Name:      nsn.Name,
	}
}
