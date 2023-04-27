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
	"net/netip"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

type Storage[alloc *ipamv1alpha1.IPAllocation, entry map[string]labels.Set] interface {
	Get() backend.Storage[alloc, entry]
}

type storageConfig struct {
	client   client.Client
	cache    backend.Cache[*table.RIB]
	runtimes Runtimes
}

func newCMStorage[alloc *ipamv1alpha1.IPAllocation, entry map[string]labels.Set](cfg *storageConfig) (Storage[alloc, entry], error) {
	r := &cm[alloc, entry]{
		cache:    cfg.cache,
		runtimes: cfg.runtimes,
	}

	be, err := backend.NewCMBackend[alloc, entry](&backend.CMConfig{
		Client:      cfg.client,
		GetData:     r.GetData,
		RestoreData: r.RestoreData,
	})
	if err != nil {
		return nil, err
	}

	r.be = be

	return r, nil
}

type cm[alloc *ipamv1alpha1.IPAllocation, entry map[string]labels.Set] struct {
	c        client.Client
	be       backend.Storage[alloc, entry]
	cache    backend.Cache[*table.RIB]
	runtimes Runtimes
	l        logr.Logger
}

func (r *cm[alloc, entry]) Get() backend.Storage[alloc, entry] {
	return r.be
}

func (r *cm[alloc, entry]) GetData(ctx context.Context, ref corev1.ObjectReference) ([]byte, error) {
	rib, err := r.cache.Get(ref, false)
	if err != nil {
		r.l.Error(err, "cannot get db info")
		return nil, err
	}

	data := map[string]labels.Set{}
	for _, route := range rib.GetTable() {
		data[route.Prefix().String()] = route.Labels()
	}
	b, err := yaml.Marshal(data)
	if err != nil {
		r.l.Error(err, "cannot marshal data")
	}
	return b, nil
}

func (r *cm[alloc, entry]) RestoreData(ctx context.Context, ref corev1.ObjectReference, cm *corev1.ConfigMap) error {
	allocations := map[string]labels.Set{}
	if err := yaml.Unmarshal([]byte(cm.Data["ipam"]), &allocations); err != nil {
		r.l.Error(err, "unmarshal error from configmap data")
		return err
	}
	r.l.Info("restored data", "ref", ref, "allocations", allocations)

	rib, err := r.cache.Get(ref, true)
	if err != nil {
		return err
	}

	// we restore in order right now
	// 1st network instance
	// 2nd prefixes
	// 3rd allocations
	r.restorePrefixes(ctx, rib, allocations, &ipamv1alpha1.NetworkInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		},
	})

	// get the prefixes from the k8s api system
	ipPrefixList := &ipamv1alpha1.IPPrefixList{}
	if err := r.c.List(context.Background(), ipPrefixList); err != nil {
		return errors.Wrap(err, "cannot get ip prefix list")
	}
	r.restorePrefixes(ctx, rib, allocations, ipPrefixList)

	// list all allocation to restore them in the ipam upon restart
	// this is the list of allocations that uses the k8s API
	ipAllocationList := &ipamv1alpha1.IPAllocationList{}
	if err := r.c.List(context.Background(), ipAllocationList); err != nil {
		return errors.Wrap(err, "cannot get ip allocation list")
	}
	r.restorePrefixes(ctx, rib, allocations, ipAllocationList)

	return nil
}

func (r *cm[alloc, entry]) restorePrefixes(ctx context.Context, rib *table.RIB, allocations map[string]labels.Set, input any) {
	var ownerGVK string
	var restoreFunc func(ctx context.Context, rib *table.RIB, prefix string, labels labels.Set, specData any)
	switch input.(type) {
	case *ipamv1alpha1.NetworkInstance:
		ownerGVK = ipamv1alpha1.NetworkInstanceKindGVKString
		restoreFunc = r.restoreNetworkInstancePrefixes
	case *ipamv1alpha1.IPPrefixList:
		ownerGVK = ipamv1alpha1.IPPrefixKindGVKString
		restoreFunc = r.restoreIPPrefixes
	case *ipamv1alpha1.IPAllocationList:
		ownerGVK = ipamv1alpha1.IPAllocationKindGVKString
		restoreFunc = r.restorIPAllocations
	default:
		r.l.Error(fmt.Errorf("expecting networkInstance, ipprefixList or ipALlocaationList, got %T", reflect.TypeOf(input)), "unexpected input data to restore")
	}

	// walk over the allocations
	for prefix, labels := range allocations {
		r.l.Info("restore allocation", "prefix", prefix, "labels", labels)
		// handle the allocation owned by the network instance
		if labels[allocv1alpha1.NephioOwnerGvkKey] == ownerGVK {
			restoreFunc(ctx, rib, prefix, labels, input)
		}
	}
}

func (r *cm[alloc, entry]) restoreNetworkInstancePrefixes(ctx context.Context, rib *table.RIB, prefix string, labels labels.Set, input any) {
	r.l = log.FromContext(ctx).WithValues("type", "niPrefixes", "prefix", prefix)
	cr, ok := input.(*ipamv1alpha1.NetworkInstance)
	if !ok {
		r.l.Error(fmt.Errorf("expecting networkInstance got %T", reflect.TypeOf(input)), "unexpected input data to restore")
		return
	}
	for _, ipPrefix := range cr.Spec.Prefixes {
		r.l.Info("restore ip prefixes", "niName", cr.GetName(), "ipPrefix", ipPrefix.Prefix)
		// the prefix is implicitly checked based on the name
		if labels[allocv1alpha1.NephioNsnNameKey] == cr.GetNameFromNetworkInstancePrefix(ipPrefix.Prefix) &&
			labels[allocv1alpha1.NephioNsnNamespaceKey] == cr.Namespace {

			if prefix != ipPrefix.Prefix {
				r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
					"mismatch prefixes",
					"kind", "aggregate",
					"stored prefix", prefix,
					"spec prefix", ipPrefix.Prefix)
			}

			rib.Add(table.NewRoute(netip.MustParsePrefix(prefix), labels, map[string]any{}))
		}
	}
}

func (r *cm[alloc, entry]) restoreIPPrefixes(ctx context.Context, rib *table.RIB, prefix string, labels labels.Set, input any) {
	r.l = log.FromContext(ctx).WithValues("type", "ipprefixes", "prefix", prefix)
	ipPrefixList, ok := input.(*ipamv1alpha1.IPPrefixList)
	if !ok {
		r.l.Error(fmt.Errorf("expecting IPPrefixList got %T", reflect.TypeOf(input)), "unexpected input data to restore")
		return
	}
	for _, ipPrefix := range ipPrefixList.Items {
		r.l.Info("restore ip prefixes", "ipPrefixName", ipPrefix.GetName(), "ipPrefix", ipPrefix.Spec.Prefix)
		if labels[allocv1alpha1.NephioNsnNameKey] == ipPrefix.GetName() &&
			labels[allocv1alpha1.NephioNsnNamespaceKey] == ipPrefix.GetNamespace() {

			// prefixes of prefixkind network need to be expanded in the subnet
			// we compare against the expanded list
			if ipPrefix.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
				pi, err := iputil.New(ipPrefix.Spec.Prefix)
				if err != nil {
					r.l.Error(err, "cannot parse prefix, should not happen since this was already stored after parsing")
					break
				}
				if !pi.IsPrefixPresentInSubnetMap(prefix) {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", ipPrefix.Spec.Kind,
						"stored prefix", prefix,
						"spec prefix", ipPrefix.Spec.Prefix)
				}

			} else {
				if prefix != ipPrefix.Spec.Prefix {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", ipPrefix.Spec.Kind,
						"stored prefix", prefix,
						"spec prefix", ipPrefix.Spec.Prefix)
				}
			}

			rib.Add(table.NewRoute(netip.MustParsePrefix(prefix), labels, map[string]any{}))
		}
	}
}

func (r *cm[alloc, entry]) restorIPAllocations(ctx context.Context, rib *table.RIB, prefix string, labels labels.Set, input any) {
	r.l = log.FromContext(ctx).WithValues("type", "ipAllocations", "prefix", prefix)
	ipAllocationList, ok := input.(*ipamv1alpha1.IPAllocationList)
	if !ok {
		r.l.Error(fmt.Errorf("expecting IPAllocationList got %T", reflect.TypeOf(input)), "unexpected input data to restore")
		return
	}
	for _, alloc := range ipAllocationList.Items {
		r.l.Info("restore ipAllocation", "alloc", alloc.GetName(), "prefix", alloc.Spec.Prefix)
		if labels[allocv1alpha1.NephioNsnNameKey] == alloc.GetName() &&
			labels[allocv1alpha1.NephioNsnNamespaceKey] == alloc.GetNamespace() {

			// for allocations the prefix can be defined in the spec or in the status
			// we want to make the next logic uniform
			allocPrefix := alloc.Spec.Prefix
			if alloc.Spec.Prefix == nil {
				allocPrefix = alloc.Status.Prefix
			}

			// prefixes of prefixkind network need to be expanded in the subnet
			// we compare against the expanded list
			if alloc.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
				// TODO this can error if the prefix got released since ipam was not available -> to be added to the allocation controller
				pi, err := iputil.New(*allocPrefix)
				if err != nil {
					r.l.Error(err, "cannot parse prefix, should not happen since this was already stored after parsing, unless the prefix got released")
					break
				}
				if !pi.IsPrefixPresentInSubnetMap(prefix) {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", alloc.Spec.Kind,
						"stored prefix", prefix,
						"alloc prefix", allocPrefix)
				}

			} else {
				if prefix != *allocPrefix {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", alloc.Spec.Kind,
						"stored prefix", prefix,
						"alloc prefix", allocPrefix)
				}
			}

			rib.Add(table.NewRoute(netip.MustParsePrefix(prefix), labels, map[string]any{}))
		}
	}
}
