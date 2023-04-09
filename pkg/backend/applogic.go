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

package backend

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AppLogic[T1 client.Object] interface {
	Get(ctx context.Context, a T1) (T1, error)
	Validate(ctx context.Context, a T1) (string, error)
	Apply(ctx context.Context, a T1) (T1, error)
	Delete(ctx context.Context, a T1) error
}

type GetHandler[T1 client.Object] func(context.Context, T1) (T1, error)
type ValidateHandler[T1 client.Object] func(context.Context, T1) (string, error)
type ApplyHandler[T1 client.Object] func(context.Context, T1) (T1, error)
type DeleteHandler[T1 client.Object] func(context.Context, T1) error

type ApplogicConfig[T1 client.Object] struct {
	GetHandler      GetHandler[T1]
	ValidateHandler ValidateHandler[T1]
	ApplyHandler    ApplyHandler[T1]
	DeleteHandler   DeleteHandler[T1]
}

func NewApplogic[T1 client.Object](cfg *ApplogicConfig[T1]) (AppLogic[T1], error) {
	if cfg.GetHandler == nil {
		return nil, fmt.Errorf("cannot create applogic without a get handler")
	}
	if cfg.ValidateHandler == nil {
		return nil, fmt.Errorf("cannot create applogic without a validate handler")
	}
	if cfg.ApplyHandler == nil {
		return nil, fmt.Errorf("cannot create applogic without a apply handler")
	}
	if cfg.DeleteHandler == nil {
		return nil, fmt.Errorf("cannot create applogic without a delete handler")
	}
	return &applogic[T1]{
		cfg: cfg,
	}, nil
}

type applogic[T1 client.Object] struct {
	cfg *ApplogicConfig[T1]
	l   logr.Logger
}

func (r *applogic[T1]) Get(ctx context.Context, a T1) (T1, error) {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("get")

	return r.cfg.GetHandler(ctx, a)

}
func (r *applogic[T1]) Validate(ctx context.Context, a T1) (string, error) {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("validate")

	return r.cfg.ValidateHandler(ctx, a)

}
func (r *applogic[T1]) Apply(ctx context.Context, a T1) (T1, error) {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("apply")

	return r.cfg.ApplyHandler(ctx, a)
}

func (r *applogic[T1]) Delete(ctx context.Context, a T1) error {
	r.l = log.FromContext(ctx).WithValues("name", a.GetName())
	r.l.Info("delete")

	return r.cfg.DeleteHandler(ctx, a)
}
