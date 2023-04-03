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
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/utils/iputil"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	allocv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

type BackendType string

const (
	BackendTypeNop       BackendType = ""
	BackendTypeConfigMap BackendType = "configmap"
)

type Backend interface {
	Restore(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
	Store(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error
	Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error
}

type BackendConfig struct {
	client   client.Client
	ipamRib  ipamRib
	runtimes Runtimes
}

func NewConfigMapBackend(c *BackendConfig) Backend {
	return &cm{
		c:        c.client,
		ipamRib:  c.ipamRib,
		runtimes: c.runtimes,
	}
}

type cm struct {
	c        client.Client
	ipamRib  ipamRib
	runtimes Runtimes
	l        logr.Logger
	//m             sync.Mutex
}

func (r *cm) Store(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error {
	r.l.Info("store")
	niRef := alloc.GetNetworkInstance()
	rib, err := r.ipamRib.getRIB(niRef, false)
	if err != nil {
		return err
	}

	cm := buildConfigMap(niRef)

	// TODO how to get the network instance namespace
	if err := r.c.Get(ctx, types.NamespacedName{Namespace: niRef.Namespace, Name: niRef.Name}, cm); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrap(r.c.Create(ctx, cm), "cannot create configmap")
		}
		r.l.Error(err, "cannot get configmap", "niRef", niRef)
	}

	data := map[string]labels.Set{}
	for _, route := range rib.GetTable() {
		data[route.Prefix().String()] = route.Labels()

	}
	b, err := yaml.Marshal(data)
	if err != nil {
		r.l.Error(err, "cannot marshal data")
	}
	cm.Data = map[string]string{}
	cm.Data["ipam"] = string(b)
	return r.c.Update(ctx, cm)
}

func (r *cm) Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	r.l = log.FromContext(ctx)
	niRef := corev1.ObjectReference{Namespace: cr.GetNamespace(), Name: cr.GetName()}
	cm := buildConfigMap(niRef)
	if err := r.c.Delete(ctx, cm); err != nil {
		if !kerrors.IsNotFound(err) {
			r.l.Error(err, "ipam delete instance cm", "name", cr.GetName())
		}
	}
	return nil
}

func (r *cm) Restore(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error {
	r.l = log.FromContext(ctx)
	niRef := corev1.ObjectReference{Namespace: cr.GetNamespace(), Name: cr.GetName()}
	r.l.Info("restore", "niRef", niRef)

	cm := buildConfigMap(niRef)
	if err := r.c.Get(ctx, types.NamespacedName{Name: niRef.Name, Namespace: niRef.Namespace}, cm); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrap(r.c.Create(ctx, cm), "canno create configmap")
		}
	}
	//allocations := map[string]map[string]string{}
	allocations := map[string]labels.Set{}
	if err := yaml.Unmarshal([]byte(cm.Data["ipam"]), &allocations); err != nil {
		r.l.Error(err, "unmarshal error from configmap data")
		return err
	}
	r.l.Info("restored data", "niRef", niRef, "allocations", allocations)

	// INFO: dynamic allocations which dont come through the k8s api
	// are resstored from the proxy cache, we assume the grpc client takes care of that

	rib, err := r.ipamRib.getRIB(niRef, true)
	if err != nil {
		return err
	}
	// we restore in order right now
	// 1st network instance
	// 2nd prefixes
	// 3rd allocations
	r.restorePrefixes(ctx, rib, allocations, cr)

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

func (r *cm) restorePrefixes(ctx context.Context, rib *table.RIB, allocations map[string]labels.Set, input any) {
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

func (r *cm) restoreNetworkInstancePrefixes(ctx context.Context, rib *table.RIB, prefix string, labels labels.Set, input any) {
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

func (r *cm) restoreIPPrefixes(ctx context.Context, rib *table.RIB, prefix string, labels labels.Set, input any) {
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
			if ipPrefix.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
				pi, err := iputil.New(ipPrefix.Spec.Prefix)
				if err != nil {
					r.l.Error(err, "cannot parse prefix, should not happen since this was already stored after parsing")
					break
				}
				if !pi.IsPrefixPresentInSubnetMap(prefix) {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", ipPrefix.Spec.PrefixKind,
						"stored prefix", prefix,
						"spec prefix", ipPrefix.Spec.Prefix)
				}

			} else {
				if prefix != ipPrefix.Spec.Prefix {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", ipPrefix.Spec.PrefixKind,
						"stored prefix", prefix,
						"spec prefix", ipPrefix.Spec.Prefix)
				}
			}

			rib.Add(table.NewRoute(netip.MustParsePrefix(prefix), labels, map[string]any{}))
		}
	}
}

func (r *cm) restorIPAllocations(ctx context.Context, rib *table.RIB, prefix string, labels labels.Set, input any) {
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
			if alloc.Spec.Prefix == "" {
				allocPrefix = alloc.Status.AllocatedPrefix
			}

			// prefixes of prefixkind network need to be expanded in the subnet
			// we compare against the expanded list
			if alloc.GetPrefixKind() == ipamv1alpha1.PrefixKindNetwork {
				// TODO this can error if the prefix got released since ipam was not available -> to be added to the allocation controller
				pi, err := iputil.New(allocPrefix)
				if err != nil {
					r.l.Error(err, "cannot parse prefix, should not happen since this was already stored after parsing, unless the prefix got released")
					break
				}
				if !pi.IsPrefixPresentInSubnetMap(prefix) {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", alloc.GetPrefixKind(),
						"stored prefix", prefix,
						"alloc prefix", allocPrefix)
				}

			} else {
				if prefix != allocPrefix {
					r.l.Error(fmt.Errorf("strange that the prefixes dont match"),
						"mismatch prefixes",
						"kind", alloc.GetPrefixKind(),
						"stored prefix", prefix,
						"alloc prefix", allocPrefix)
				}
			}

			rib.Add(table.NewRoute(netip.MustParsePrefix(prefix), labels, map[string]any{}))
		}
	}
}

func buildConfigMap(niRef corev1.ObjectReference) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: niRef.Namespace,
			Name:      niRef.Name,
			//ResourceVersion: "",
		},
	}
}

func NewNopBackend() Backend {
	return &nopBackend{}
}

type nopBackend struct{}

func (r *nopBackend) Restore(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error { return nil }
func (r *nopBackend) Store(ctx context.Context, alloc *ipamv1alpha1.IPAllocation) error   { return nil }
func (r *nopBackend) Delete(ctx context.Context, cr *ipamv1alpha1.NetworkInstance) error  { return nil }
