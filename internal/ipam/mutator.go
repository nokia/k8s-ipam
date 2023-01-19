package ipam

/*
import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Mutator interface {
	MutateAllocWithoutPrefix(ctx context.Context) []*ipamv1alpha1.IPAllocation
	MutateAllocWithPrefix(ctx context.Context) []*ipamv1alpha1.IPAllocation
	MutateAllocNetworkWithPrefix(ctx context.Context) []*ipamv1alpha1.IPAllocation
}

type MutatorConfig struct {
	alloc *ipamv1alpha1.IPAllocation
	pi    iputil.PrefixInfo
}

func NewMutator(c *MutatorConfig) Mutator {
	return &mutator{
		alloc: c.alloc,
		pi:    c.pi,
	}
}

type mutator struct {
	alloc *ipamv1alpha1.IPAllocation
	pi    iputil.PrefixInfo
	l     logr.Logger
}

func (r *mutator) MutateAllocWithoutPrefix(ctx context.Context) []*ipamv1alpha1.IPAllocation {
	r.l = log.FromContext(ctx)
	if r.alloc.GetPrefix() != "" {
		r.l.Error(fmt.Errorf("mutating alloc without prefix, but prefix exists: %s", r.alloc.GetPrefix()), "unexpected fn call")
	}

	newallocs := []*ipamv1alpha1.IPAllocation{}
	// copy allocation
	newalloc := r.alloc.DeepCopy()
	if len(newalloc.Spec.Labels) == 0 {
		newalloc.Spec.Labels = map[string]string{}
	}
	// Add prefix kind key
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixKindKey] = string(newalloc.GetPrefixKind())
	// Add address family key
	//newalloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(newalloc.GetAddressFamily())
	// add ip pool labelKey if present
	if newalloc.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
		newalloc.Spec.Labels[ipamv1alpha1.NephioPoolKey] = "true"
	}
	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)
	if r.alloc.GetPrefixLengthFromSpec() != 0 {
		newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = strconv.Itoa(int(r.alloc.GetPrefixLengthFromSpec()))
	}
	// build the generic allocation
	newallocs = append(newallocs, newalloc)
	return newallocs
}

func (r *mutator) MutateAllocWithPrefix(ctx context.Context) []*ipamv1alpha1.IPAllocation {
	r.l = log.FromContext(ctx)
	if r.alloc.GetPrefix() == "" {
		r.l.Error(fmt.Errorf("mutating alloc with prefix, but no prefix exists: %s", r.alloc.GetPrefix()), "unexpected fn call")
	}

	newallocs := []*ipamv1alpha1.IPAllocation{}
	// copy allocation
	newalloc := r.alloc.DeepCopy()
	//newlabels := newalloc.GetLabels()
	if len(newalloc.Spec.Labels) == 0 {
		newalloc.Spec.Labels = map[string]string{}
	}
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixKindKey] = string(newalloc.GetPrefixKind())
	newalloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(r.pi.GetAddressFamily())

	// add ip pool labelKey if present
	if newalloc.GetPrefixKind() == ipamv1alpha1.PrefixKindPool {
		newalloc.Spec.Labels[ipamv1alpha1.NephioPoolKey] = "true"
	}
	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()
	newalloc.Spec.Labels[ipamv1alpha1.NephioSubnetKey] = r.pi.GetSubnetName()
	if newalloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
		//newalloc.Spec.Labels[ipamv1alpha1.NephioParentPrefixLengthKey] = r.pi.GetPrefixLength().String()
	}
	// build the generic allocation
	newallocs = append(newallocs, newalloc)

	return newallocs
}

func (r *mutator) MutateAllocNetworkWithPrefix(ctx context.Context) []*ipamv1alpha1.IPAllocation {
	r.l = log.FromContext(ctx)
	if r.alloc.GetPrefix() == "" && r.alloc.GetPrefixKind() != ipamv1alpha1.PrefixKindNetwork {
		r.l.Error(fmt.Errorf("mutating alloc with prefix and prefixkind network, got prefix: %s, prefixKinf: %s", r.alloc.GetPrefix(), r.alloc.GetPrefixKind()), "unexpected fn call")
	}
	newallocs := []*ipamv1alpha1.IPAllocation{}
	// prepare additional labels generically
	if len(r.alloc.Spec.Labels) == 0 {
		r.alloc.Spec.Labels = map[string]string{}
	}
	r.alloc.Spec.Labels[ipamv1alpha1.NephioPrefixKindKey] = string(r.alloc.GetPrefixKind())
	r.alloc.Spec.Labels[ipamv1alpha1.NephioAddressFamilyKey] = string(r.pi.GetAddressFamily())
	r.alloc.Spec.Labels[ipamv1alpha1.NephioSubnetKey] = r.pi.GetSubnetName()
	//r.alloc.Spec.Labels[ipamv1alpha1.NephioParentPrefixLengthKey] = r.pi.GetPrefixLength().String()
	// NO POOL allowed here	delete(alloc.Spec.Labels, ipamv1alpha1.NephioPoolKey)

	if r.pi.IsNorLastNorFirst() {
		// allocate the network
		newallocs = append(newallocs, r.mutateNetworkNet(r.alloc))
		// allocate the address
		newallocs = append(newallocs, r.mutateNetworkIPAddress(r.alloc))
		// allocate the first address)
		newallocs = append(newallocs, r.mutateNetworkFirstAddressInNet(r.alloc))
		// allocate the last address
		newallocs = append(newallocs, r.mutateNetworkLastAddressInNet(r.alloc))
	}
	if r.pi.IsFirst() {
		// allocate the network part
		newallocs = append(newallocs, r.mutateNetworkNet(r.alloc))
		// allocate the address part, which is the first address in this scenario
		newallocs = append(newallocs, r.mutateNetworkIPAddress(r.alloc))
		// allocate the last address
		newallocs = append(newallocs, r.mutateNetworkLastAddressInNet(r.alloc))
	}

	if r.pi.IsLast() {
		// allocate the network part
		newallocs = append(newallocs, r.mutateNetworkNet(r.alloc))
		// allocate the address part, which is the last address in this scenario
		newallocs = append(newallocs, r.mutateNetworkIPAddress(r.alloc))
		// allocate the last address
		newallocs = append(newallocs, r.mutateNetworkFirstAddressInNet(r.alloc))
	}

	return newallocs
}

func (r *mutator) mutateNetworkIPAddress(alloc *ipamv1alpha1.IPAllocation) *ipamv1alpha1.IPAllocation {
	// copy allocation
	newalloc := alloc.DeepCopy()
	// set address
	newalloc.Spec.Prefix = r.pi.GetIPAddressPrefix().String()
	// set address prefix length
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()
	return newalloc
}

func (r *mutator) mutateNetworkNet(alloc *ipamv1alpha1.IPAllocation) *ipamv1alpha1.IPAllocation {
	// copy allocation
	newalloc := alloc.DeepCopy()
	newalloc.Spec.Prefix = r.pi.GetIPSubnet().String()
	//newalloc.Name = r.pi.GetSubnetName()
	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)
	//newalloc.Spec.Labels[ipamv1alpha1.NephioNsnNameKey] = r.pi.GetSubnetName() + "-" + ipamv1alpha1.SubnetPrefix
	//newalloc.Spec.Labels[ipamv1alpha1.NephioNsnNamespaceKey] = alloc.Namespace
	//newalloc.Spec.Labels[ipamv1alpha1.NephioGvkKey] = ipamv1alpha1.OriginSystem
	//newalloc.Spec.Labels[ipamv1alpha1.NephioIPContributingRouteKey] = r.alloc.GetGenericNamespacedName()
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetPrefixLength().String()
	return newalloc
}

func (r *mutator) mutateNetworkFirstAddressInNet(alloc *ipamv1alpha1.IPAllocation) *ipamv1alpha1.IPAllocation {
	// copy allocation
	newalloc := alloc.DeepCopy()
	newalloc.Spec.Prefix = r.pi.GetFirstIPPrefix().String()
	newalloc.Name = r.pi.GetFirstIPAddress().String()
	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)
	//newalloc.Spec.Labels[ipamv1alpha1.NephioNsnNameKey] = r.pi.GetSubnetName() + "-" + ipamv1alpha1.SubnetFirstAddress
	//newalloc.Spec.Labels[ipamv1alpha1.NephioNsnNamespaceKey] = alloc.Namespace
	//newalloc.Spec.Labels[ipamv1alpha1.NephioGvkKey] = ipamv1alpha1.OriginSystem
	//newalloc.Spec.Labels[ipamv1alpha1.NephioIPContributingRouteKey] = r.alloc.GetGenericNamespacedName()
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()
	return newalloc
}

func (r *mutator) mutateNetworkLastAddressInNet(alloc *ipamv1alpha1.IPAllocation) *ipamv1alpha1.IPAllocation {
	// copy allocation
	newalloc := alloc.DeepCopy()
	newalloc.Spec.Prefix = r.pi.GetLastIPPrefix().String()
	newalloc.Name = r.pi.GetLastIPAddress().String()
	// NO GW allowed here
	delete(newalloc.Spec.Labels, ipamv1alpha1.NephioGatewayKey)
	//newalloc.Spec.Labels[ipamv1alpha1.NephioNsnNameKey] = r.pi.GetSubnetName() + "-" + ipamv1alpha1.SubnetLastAddress
	//newalloc.Spec.Labels[ipamv1alpha1.NephioNsnNamespaceKey] = alloc.Namespace
	//newalloc.Spec.Labels[ipamv1alpha1.NephioGvkKey] = ipamv1alpha1.OriginSystem
	//newalloc.Spec.Labels[ipamv1alpha1.NephioIPContributingRouteKey] = r.alloc.GetGenericNamespacedName()
	newalloc.Spec.Labels[ipamv1alpha1.NephioPrefixLengthKey] = r.pi.GetAddressPrefixLength().String()

	return newalloc
}
*/
