package backend

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const ConfigMapKey = "data"

type GetDataFn func(ctx context.Context, ref corev1.ObjectReference) ([]byte, error)
type RestoreDataFn func(ctx context.Context, ref corev1.ObjectReference, cm *corev1.ConfigMap) error

type CMConfig struct {
	Client      client.Client
	GetData     GetDataFn
	RestoreData RestoreDataFn
	Prefix      string
}

func NewCMBackend[alloc, entry any](cfg *CMConfig) (Storage[alloc, entry], error) {
	if cfg.GetData == nil {
		return nil, fmt.Errorf("get data callback fn is required")
	}
	if cfg.RestoreData == nil {
		return nil, fmt.Errorf("restore data callback fn is required")
	}

	return &cm[alloc, entry]{
		c:      cfg.Client,
		cfg:    cfg,
		prefix: cfg.Prefix,
	}, nil
}

type cm[alloc, entry any] struct {
	c      client.Client
	cfg    *CMConfig
	prefix string
	l      logr.Logger
}

func (r *cm[alloc, entry]) Restore(ctx context.Context, ref corev1.ObjectReference) error {
	r.l = log.FromContext(ctx)
	r.l.Info("restore", "indexRef", ref)

	// if no client provided dont try to restore
	if r.c == nil {
		return nil
	}

	cm := buildConfigMap(ref, r.prefix)
	if err := r.c.Get(ctx, types.NamespacedName{Name: r.prefix + "-" + ref.Name, Namespace: ref.Namespace}, cm); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrap(r.c.Create(ctx, cm), "cannot create configmap")
		}
	}

	// call the callback Fn
	if err := r.cfg.RestoreData(ctx, ref, cm); err != nil {
		r.l.Error(err, "cannot resore data")
		return err
	}

	return nil
}

// only used in configmap
func (r *cm[alloc, entry]) SaveAll(ctx context.Context, ref corev1.ObjectReference) error {
	r.l = log.FromContext(ctx)

	// if no client provided dont try to save
	if r.c == nil {
		return nil
	}

	cm := buildConfigMap(ref, r.prefix)

	if err := r.c.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: r.prefix + "-" + ref.Name}, cm); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrap(r.c.Create(ctx, cm), "cannot create configmap")
		}
		r.l.Error(err, "cannot get configmap", "ref", ref)
	}

	// call the callback Fn
	b, err := r.cfg.GetData(ctx, ref)
	if err != nil {
		r.l.Error(err, "cannot marshal data")
		return err
	}

	cm.Data = map[string]string{}
	cm.Data[ConfigMapKey] = string(b)
	return r.c.Update(ctx, cm)
}

func (r *cm[alloc, entry]) Destroy(ctx context.Context, ref corev1.ObjectReference) error {
	r.l = log.FromContext(ctx)
	// if no client provided dont try to save
	if r.c == nil {
		return nil
	}
	cm := buildConfigMap(ref, r.prefix)
	if err := r.c.Delete(ctx, cm); err != nil {
		if !kerrors.IsNotFound(err) {
			r.l.Error(err, "ipam delete instance cm", "name", ref.Name)
		}
	}
	return nil
}

// Get is a getall actually
func (r *cm[alloc, entry]) Get(ctx context.Context, a alloc) ([]entry, error) {
	return nil, nil
}

func (r *cm[alloc, entry]) Set(ctx context.Context, a alloc) error {
	return nil
}

func (r *cm[alloc, entry]) Delete(ctx context.Context, a alloc) error {
	return nil
}

func buildConfigMap(indexRef corev1.ObjectReference, prefix string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: indexRef.Namespace,
			Name:      prefix + "-" + indexRef.Name,
		},
	}
}
