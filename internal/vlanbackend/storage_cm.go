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

package vlanbackend

import (
	"context"

	"github.com/go-logr/logr"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/pkg/backend"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Storage[alloc *vlanv1alpha1.VLANAllocation, entry map[string]labels.Set] interface {
	Get() backend.Storage[alloc, entry]
}

type storageConfig struct {
	client client.Client
	cache  backend.Cache[db.DB[uint16]]
}

func newCMStorage[alloc *vlanv1alpha1.VLANAllocation, entry map[string]labels.Set](cfg *storageConfig) (Storage[alloc, entry], error) {
	r := &cm[alloc, entry]{
		cache: cfg.cache,
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

type cm[alloc *vlanv1alpha1.VLANAllocation, entry map[string]labels.Set] struct {
	be    backend.Storage[alloc, entry]
	cache backend.Cache[db.DB[uint16]]
	l     logr.Logger
}

func (r *cm[alloc, entry]) Get() backend.Storage[alloc, entry] {
	return r.be
}

func (r *cm[alloc, entry]) GetData(ctx context.Context, ref corev1.ObjectReference) ([]byte, error) {
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

func (r *cm[alloc, entry]) RestoreData(ctx context.Context, ref corev1.ObjectReference, cm *corev1.ConfigMap) error {
	allocations := map[uint16]labels.Set{}
	if err := yaml.Unmarshal([]byte(cm.Data["ipam"]), &allocations); err != nil {
		r.l.Error(err, "unmarshal error from configmap data")
		return err
	}
	r.l.Info("restored data", "ref", ref, "allocations", allocations)

	ca, err := r.cache.Get(ref, true)
	if err != nil {
		return err
	}

	for vlanID, labels := range allocations {
		ca.Set(db.NewEntry(vlanID, labels))
	}
	return nil
}
