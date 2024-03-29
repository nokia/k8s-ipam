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

package vlan

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/db"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

type Storage interface {
	Get() backend.Storage[*vlanv1alpha1.VLANClaim, map[string]labels.Set]
}

type storageConfig struct {
	client client.Client
	cache  backend.Cache[db.DB[uint16]]
}

func newCMStorage(cfg *storageConfig) (Storage, error) {
	r := &cm{
		c:     cfg.client,
		cache: cfg.cache,
	}

	be, err := backend.NewCMBackend[*vlanv1alpha1.VLANClaim, map[string]labels.Set](&backend.CMConfig{
		Client:      cfg.client,
		GetData:     r.GetData,
		RestoreData: r.RestoreData,
		Prefix:      "vlan",
	})
	if err != nil {
		return nil, err
	}

	r.be = be

	return r, nil
}

type cm struct {
	c     client.Client
	be    backend.Storage[*vlanv1alpha1.VLANClaim, map[string]labels.Set]
	cache backend.Cache[db.DB[uint16]]
	l     logr.Logger
}

func (r *cm) Get() backend.Storage[*vlanv1alpha1.VLANClaim, map[string]labels.Set] {
	return r.be
}

func (r *cm) GetData(ctx context.Context, ref corev1.ObjectReference) ([]byte, error) {
	r.l = log.FromContext(ctx)
	ca, err := r.cache.Get(ref, false)
	if err != nil {
		r.l.Error(err, "cannot get db info")
		return nil, err
	}

	data := map[uint16]labels.Set{}
	for _, entry := range ca.GetAll() {
		data[entry.ID()] = entry.Labels()
	}
	b, err := yaml.Marshal(data)
	if err != nil {
		r.l.Error(err, "cannot marshal data")
	}
	return b, nil
}

func (r *cm) RestoreData(ctx context.Context, ref corev1.ObjectReference, cm *corev1.ConfigMap) error {
	r.l = log.FromContext(ctx)
	claims := map[uint16]labels.Set{}
	if err := yaml.Unmarshal([]byte(cm.Data[backend.ConfigMapKey]), &claims); err != nil {
		r.l.Error(err, "unmarshal error from configmap data")
		return err
	}
	r.l.Info("restore data", "ref", ref, "claims", claims)

	// Get
	ca, err := r.cache.Get(ref, true)
	if err != nil {
		return err
	}

	vlanList := &vlanv1alpha1.VLANList{}
	if err := r.c.List(context.Background(), vlanList); err != nil {
		return errors.Wrap(err, "cannot get vlan list")
	}
	r.restoreVLANs(ctx, ca, claims, vlanList)

	vlanClaimList := &vlanv1alpha1.VLANClaimList{}
	if err := r.c.List(context.Background(), vlanClaimList); err != nil {
		return errors.Wrap(err, "cannot get vlan claim list")
	}
	r.restoreVLANs(ctx, ca, claims, vlanClaimList)

	return nil
}

func (r *cm) restoreVLANs(ctx context.Context, ca db.DB[uint16], claims map[uint16]labels.Set, input any) {
	var ownerGVK string
	var restoreFunc func(ctx context.Context, ca db.DB[uint16], vlanID uint16, labels labels.Set, specData any)
	switch input.(type) {
	case *vlanv1alpha1.VLANList:
		ownerGVK = vlanv1alpha1.VLANKindGVKString
		restoreFunc = r.restoreStaticVLANs
	case *vlanv1alpha1.VLANClaimList:
		ownerGVK = vlanv1alpha1.VLANClaimKindGVKString
		restoreFunc = r.restoreDynamicVLANs
	default:
		r.l.Error(fmt.Errorf("expecting vlanIndex, vlanList or ipClaimList, got %T", reflect.TypeOf(input)), "unexpected input data to restore")
	}
	for vlanID, labels := range claims {
		r.l.Info("restore claims", "vlanID", vlanID, "labels", labels)
		// handle the claims owned by the network instance
		if labels[resourcev1alpha1.NephioOwnerGvkKey] == ownerGVK {
			restoreFunc(ctx, ca, vlanID, labels, input)
		}
	}
}

func (r *cm) restoreStaticVLANs(ctx context.Context, ca db.DB[uint16], vlanID uint16, labels labels.Set, input any) {
	r.l = log.FromContext(ctx).WithValues("type", "staticVLANs", "vlanID", vlanID)
	vlanList, ok := input.(*vlanv1alpha1.VLANList)
	if !ok {
		r.l.Error(fmt.Errorf("expecting VLANList got %T", reflect.TypeOf(input)), "unexpected input data to restore")
		return
	}
	for _, vlan := range vlanList.Items {
		r.l.Info("restore static VLANs", "vlanName", vlan.GetName(), "vlanID", vlan.Spec.VLANID)
		if labels[resourcev1alpha1.NephioNsnNameKey] == vlan.GetName() &&
			labels[resourcev1alpha1.NephioNsnNamespaceKey] == vlan.GetNamespace() {

			if vlanID != *vlan.Spec.VLANID {
				// could happen if the db is initializing
				r.l.Error(fmt.Errorf("strange that the vlanID(S) dont match"),
					"mismatch vlanIDs",
					"stored vlanID", vlanID,
					"spec vlanID", vlan.Spec.VLANID)
			}
			r.l.Info("restored Static VLAN", "VLANID", vlanID)
			ca.Set(db.NewEntry(vlanID, labels))
		}
	}
}

func (r *cm) restoreDynamicVLANs(ctx context.Context, ca db.DB[uint16], vlanID uint16, labels labels.Set, input any) {
	r.l = log.FromContext(ctx).WithValues("type", "vlanClaims", "vlanID", vlanID)
	vlanClaimList, ok := input.(*vlanv1alpha1.VLANClaimList)
	if !ok {
		r.l.Error(fmt.Errorf("expecting vlanClaimList got %T", reflect.TypeOf(input)), "unexpected input data to restore")
		return
	}
	for _, claim := range vlanClaimList.Items {
		r.l.Info("restore Dynamic cliams", "claim", claim.GetName(), "vlanID", claim.Spec.VLANID)
		if labels[resourcev1alpha1.NephioNsnNameKey] == claim.GetName() &&
			labels[resourcev1alpha1.NephioNsnNamespaceKey] == claim.GetNamespace() {

			// for claims the vlanID can be defined in the spec or in the status
			// we want to make the next logic uniform
			claimVLANID := claim.Spec.VLANID
			if claim.Spec.VLANID == nil {
				claimVLANID = claim.Status.VLANID
			}

			if claimVLANID == nil || (claimVLANID != nil && vlanID != *claimVLANID) {
				r.l.Error(fmt.Errorf("strange that the vlanID(S) dont match"),
					"mismatch vlanIDs",
					"stored vlanID", vlanID,
					"claimed vlanID", claimVLANID)
			}
			r.l.Info("restored Dynamic VLAN", "VLANID", vlanID)
			ca.Set(db.NewEntry(vlanID, labels))
		}
	}
}

func newNopCMStorage() Storage {
	return &nopcm{
		be: backend.NewNopStorage[*vlanv1alpha1.VLANClaim, map[string]labels.Set](),
	}
}

type nopcm struct {
	be backend.Storage[*vlanv1alpha1.VLANClaim, map[string]labels.Set]
}

func (r *nopcm) Get() backend.Storage[*vlanv1alpha1.VLANClaim, map[string]labels.Set] {
	return r.be
}
