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

package ipam

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/pkg/errors"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Option can be used to manipulate Options.
type Option func(Ipam)

type Ipam interface {
	// Create a virtual ipam instance
	Initialize(cr *ipamv1alpha1.NetworkInstance) error
	Create(string)
	Delete(crName string)
	Get(crName string) (*table.RouteTable, bool)
	AllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) (*string, *string, error)
	DeAllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error
}

func New(c client.Client, opts ...Option) Ipam {
	i := &ipam{
		c:    c,
		ipam: make(map[string]*table.RouteTable),
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

type ipam struct {
	c    client.Client
	m    sync.Mutex
	ipam map[string]*table.RouteTable

	l logr.Logger
}

func (r *ipam) Initialize(cr *ipamv1alpha1.NetworkInstance) error {
	r.m.Lock()
	defer r.m.Unlock()
	r.l = log.FromContext(context.Background())

	crName := types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}
	r.l = log.FromContext(context.Background())
	r.l.Info("ipam action", "action", "initialize", "name", crName, "ipam", r.ipam)

	r.ipam[crName.String()] = table.NewRouteTable()

	prefixList := &ipamv1alpha1.IPPrefixList{}
	if err := r.c.List(context.Background(), prefixList); err != nil {
		return errors.Wrap(err, "cannot get ip prefix list")
	}

	allocList := &ipamv1alpha1.IPAllocationList{}
	if err := r.c.List(context.Background(), allocList); err != nil {
		return errors.Wrap(err, "cannot get ip allocation list")
	}

	for prefix, labels := range cr.Status.Allocations {
		switch labels.Get(ipamv1alpha1.NephioOriginKey) {
		case string(ipamv1alpha1.OriginIPPrefix):
			for _, p := range prefixList.Items {
				if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == p.Name {
					if prefix != p.Spec.Prefix {
						r.l.Info("ipam action", "action", "initialize", "prefix", "match error", "ni prefix", prefix, "ipprefix prefix", p.Spec.Prefix)
					}
					a := netaddr.MustParseIPPrefix(prefix)
					route := table.NewRoute(a)
					route.UpdateLabel(labels)
					r.ipam[crName.String()].Add(route)

					r.l.Info("ipam action", "action", "initialize", "added prefix", prefix)
				}
			}
		case string(ipamv1alpha1.OriginIPAllocation):
			for _, p := range allocList.Items {
				if labels.Get(ipamv1alpha1.NephioIPAllocactionNameKey) == p.Name {
					if prefix != p.Spec.Prefix {
						r.l.Info("ipam action", "action", "initialize", "alloc", "match error", "ni prefix", prefix, "alloc prefix", p.Spec.Prefix)
					}
					a := netaddr.MustParseIPPrefix(prefix)
					route := table.NewRoute(a)
					route.UpdateLabel(labels)
					r.ipam[crName.String()].Add(route)

					r.l.Info("ipam action", "action", "initialize", "added alloc", prefix)
				}
			}
		default:
		}
	}
	return nil
}

func (r *ipam) Create(crName string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.l = log.FromContext(context.Background())
	r.l.Info("ipam action", "action", "create", "name", crName, "ipam", r.ipam)
	if _, ok := r.ipam[crName]; !ok {
		r.ipam[crName] = table.NewRouteTable()
	}
	r.l.Info("ipam action", "action", "create", "name", crName)
	r.l.Info("ipam action", "action", "create", "size", r.ipam[crName].Size())
}

func (r *ipam) Delete(crName string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.l = log.FromContext(context.Background())
	r.l.Info("ipam action", "action", "delete", "name", crName)
	delete(r.ipam, crName)
}

func (r *ipam) Get(crName string) (*table.RouteTable, bool) {
	r.m.Lock()
	defer r.m.Unlock()
	ip, ok := r.ipam[crName]
	return ip, ok
}

func (r *ipam) AllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) (*string, *string, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("allocate prefix ", "alloc", alloc)

	rt, err := r.validateIPAllocation(alloc)
	if err != nil {
		r.l.Error(err, "allocate prefix validation failed")
		return nil, nil, err
	}
	s, err := r.getSelectors(alloc)
	if err != nil {
		return nil, nil, err
	}
	parentPrefix := ""
	pPrefix := ""
	pPrefixLength := ""
	prefix := alloc.Spec.Prefix
	// The allocation request contains an ip prefix
	if prefix != "" {
		r.l.Info("allocate with prefix ", "prefix", prefix)
		a, err := netaddr.ParseIPPrefix(prefix)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot parse ip prefix")
		}
		// check if the prefix already exists
		_, ok, err := rt.Get(a)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot get ip prefix")
		}
		route := table.NewRoute(a)
		route.UpdateLabel(s.labels)

		if ok {
			// route exists

			// if the origin is a prefix we cannot let another static prefix take over
			// the entry in the routing table
			if alloc.Labels[ipamv1alpha1.NephioOriginKey] == string(ipamv1alpha1.OriginIPPrefix) {
				allocationName := route.GetLabels().Get(ipamv1alpha1.NephioIPAllocactionNameKey)
				if allocationName != alloc.GetName() {
					return nil, nil, fmt.Errorf("cannot allocate prefix, another overlapping ip prefix exists with name: %s", allocationName)
				}
				if err := rt.Update(route); err != nil {
					return nil, nil, errors.Wrap(err, "cannot update prefix")
				}
			}
		} else {
			// route does not exist
			if err := rt.Add(route); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					return nil, nil, errors.Wrap(err, "cannot add prefix")
				}
			}
		}

		prefix = route.String()
	} else {
		r.l.Info("allocate w/o prefix ")
		// check if the prefix/alloc already exists in the routing table
		// based on the name of the allocator
		routes := rt.GetByLabel(s.allocSelector)
		if len(routes) != 0 {
			// there should only be 1 route with this name in the route table
			routes[0].UpdateLabel(s.labels)
			prefix = routes[0].String()

			prefixLength := r.getPrefixLength(routes[0], alloc)
			if prefixLength == 32 || prefixLength == 128 {
				pPrefix = routes[0].GetLabels().Get(ipamv1alpha1.NephioParentNetKey)
				pPrefixLength = routes[0].GetLabels().Get(ipamv1alpha1.NephioParentPrefixLengthKey)
				if pPrefix != "" && pPrefixLength != "" {
					parentPrefix = strings.Join([]string{pPrefix, pPrefixLength}, "/")
				}
			}

		} else {
			// alloc has no prefix assigned, try to assign prefix
			routes := rt.GetByLabel(s.labelSelector)
			if len(routes) == 0 {
				// during startup when not everything is initialized
				// we break and the reconciliation will take care
				return nil, nil, fmt.Errorf("no available routes based on the label selector: %v", s.labelSelector)
			} else {
				// TBD we take the first prefix that matches allows for the prefixLength
				r.l.Info("alloc w/o prefix", "selector", s.labelSelector)
				r.l.Info("alloc w/o prefix", "routes", routes)
				prefixLength := r.getPrefixLength(routes[0], alloc)

				var selectedRoute *table.Route
				if prefixLength == 32 || prefixLength == 128 {
					selectedRoute = routes[0]

					pPrefix = selectedRoute.GetLabels().Get(ipamv1alpha1.NephioParentNetKey)
					pPrefixLength = selectedRoute.GetLabels().Get(ipamv1alpha1.NephioParentPrefixLengthKey)

					r.l.Info("parent prefix ", "route", selectedRoute.String(), "parent-prefix", pPrefix, "pPrefixLength", pPrefixLength)
					if pPrefix != "" && pPrefixLength != "" {
						parentPrefix = strings.Join([]string{pPrefix, pPrefixLength}, "/")
					} else {
						p := netaddr.MustParseIPPrefix(selectedRoute.String())

						pPrefix = strings.Split(selectedRoute.String(), "/")[0]
						//pPrefix =
						prefixSize, _ := p.IPNet().Mask.Size()
						pPrefixLength = strconv.Itoa(prefixSize)

						n := strings.Split(p.Masked().IPNet().String(), "/")
						parentPrefix = strings.Join([]string{n[0], pPrefixLength}, "/")
					}
				} else {
					for _, route := range routes {
						p := netaddr.MustParseIPPrefix(route.String())
						ps, _ := p.IPNet().Mask.Size()
						if ps < int(prefixLength) {
							selectedRoute = route
							break
						}
					}
				}

				if selectedRoute == nil {
					return nil, nil, fmt.Errorf("no route found with requested prefixLength: %d", prefixLength)
				}

				a, ok := rt.FindFreePrefix(selectedRoute.IPPrefix(), prefixLength)
				if !ok {
					return nil, nil, errors.New("no free prefix found")
				}
				route := table.NewRoute(a)
				if prefixLength == 32 || prefixLength == 128 {
					s.labels[ipamv1alpha1.NephioParentNetKey] = pPrefix
					s.labels[ipamv1alpha1.NephioParentPrefixLengthKey] = pPrefixLength
				}
				route.UpdateLabel(s.labels)
				if err := rt.Add(route); err != nil {
					if !strings.Contains(err.Error(), "already exists") {
						return nil, nil, errors.Wrap(err, "route insertion failed")
					}
				}

				prefix = route.String()
			}
		}
	}

	// update allocations based on latest routing table
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.c.Get(ctx, types.NamespacedName{
		Namespace: alloc.GetNamespace(),
		Name:      alloc.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey],
	}, ni); err != nil {
		return nil, nil, errors.Wrap(err, "cannot get network instance")
	}

	// always reinitialize the allocations based on latest info
	ni.Status.Allocations = make(map[string]labels.Set)
	for _, r := range rt.GetTable() {
		ni.Status.Allocations[r.String()] = *r.GetLabels()
	}

	r.l.Info("parent prefix ", "prefix", prefix, "parent-prefix", parentPrefix)
	return &parentPrefix, &prefix, errors.Wrap(r.c.Status().Update(ctx, ni), "cannot update ni status")
}

func (r *ipam) DeAllocateIPPrefix(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error {
	r.l = log.FromContext(ctx)
	r.l.Info("deallocate prefix ", "alloc", alloc)

	rt, err := r.validateIPAllocation(alloc)
	if err != nil {
		return err
	}

	s, err := r.getSelectors(alloc)
	if err != nil {
		return err
	}

	routes := rt.GetByLabel(s.allocSelector)
	for _, route := range routes {
		_, ok, err := rt.Delete(route)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("prefix not deleted")
		}
	}

	// update allocations based on latest routing table
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.c.Get(ctx, types.NamespacedName{
		Namespace: alloc.GetNamespace(),
		Name:      alloc.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey],
	}, ni); err != nil {
		return errors.Wrap(err, "cannot get network instance")
	}

	r.l.Info("deallocate prefix ", "len alloc", len(ni.Status.Allocations))
	// always reinitialize the allocations based on latest info
	ni.Status.Allocations = make(map[string]labels.Set)
	for _, r := range rt.GetTable() {
		ni.Status.Allocations[r.String()] = *r.GetLabels()
	}
	return errors.Wrap(r.c.Status().Update(ctx, ni), "cannot update ni status")

}

// validateIPAllocation validatess the network instance existance in the ipam
func (r *ipam) validateIPAllocation(alloc *ipamv1alpha1.IPAllocation) (*table.RouteTable, error) {
	ni, ok := alloc.Spec.Selector.MatchLabels[ipamv1alpha1.NephioNetworkInstanceKey]
	if !ok {
		return nil, fmt.Errorf("an ip allocation required a label selector key %s, which is not provided", ipamv1alpha1.NephioNetworkInstanceKey)
	}

	niName := types.NamespacedName{
		Namespace: alloc.GetNamespace(),
		Name:      ni,
	}
	rt, ok := r.Get(niName.String())
	if !ok {
		return nil, fmt.Errorf("ipam ni not ready or network-instance %s not correct", ni)
	}
	r.l.Info("validateIPAllocation", "ipam size", rt.Size())
	return rt, nil
}

func (r *ipam) getLabelSelector(alloc *ipamv1alpha1.IPAllocation) (labels.Selector, error) {
	fullselector := labels.NewSelector()
	for key, val := range alloc.Spec.Selector.MatchLabels {
		req, err := labels.NewRequirement(key, selection.In, []string{val})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
	}
	return fullselector, nil
}

func (r *ipam) getAllocSelector(alloc *ipamv1alpha1.IPAllocation) (labels.Selector, error) {
	fullselector := labels.NewSelector()
	req, err := labels.NewRequirement(ipamv1alpha1.NephioIPAllocactionNameKey, selection.In, []string{alloc.GetName()})
	if err != nil {
		return nil, err
	}
	return fullselector.Add(*req), nil
}

type selectors struct {
	fullSelector  labels.Selector
	labelSelector labels.Selector
	allocSelector labels.Selector
	labels        map[string]string
}

// getSelectors returns the labelSelector and the full selector
func (r *ipam) getSelectors(alloc *ipamv1alpha1.IPAllocation) (*selectors, error) {
	labelSelector, err := r.getLabelSelector(alloc)
	if err != nil {
		return nil, err
	}

	allocSelector, err := r.getAllocSelector(alloc)
	if err != nil {
		return nil, err
	}

	fullselector := labels.NewSelector()
	l := make(map[string]string)
	for key, val := range alloc.Spec.Selector.MatchLabels {
		req, err := labels.NewRequirement(key, selection.In, []string{val})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
		l[key] = val
	}
	for key, val := range alloc.GetLabels() {
		req, err := labels.NewRequirement(key, selection.In, []string{val})
		if err != nil {
			return nil, err
		}
		fullselector = fullselector.Add(*req)
		l[key] = val
	}
	// add the allocation Name key to the label list as we use this internally
	// for validating prefix names
	l[ipamv1alpha1.NephioIPAllocactionNameKey] = alloc.GetName()
	return &selectors{
		fullSelector:  fullselector,
		labelSelector: labelSelector,
		allocSelector: allocSelector,
		labels:        l,
	}, nil
}

func (r *ipam) getPrefixLength(route *table.Route, alloc *ipamv1alpha1.IPAllocation) uint8 {
	if alloc.Spec.PrefixLength != 0 {
		return alloc.Spec.PrefixLength
	}
	if route.IPPrefix().IP().Is4() {
		return 32
	}
	return 128
}
