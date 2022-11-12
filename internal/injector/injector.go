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

package injector

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/internal/backoff"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Injector interface {
	GetName() string
	Run(ctx context.Context)
	Stop()
}

type InjectorFn func(context.Context, types.NamespacedName) error

type Config struct {
	NamespacedName  types.NamespacedName
	InjectorHandler InjectorFn
	Client          client.Client
}

func New(c *Config) Injector {
	return &injector{
		c:              c.Client,
		namespacedName: c.NamespacedName,
		injectorFn:     c.InjectorHandler,
		retryAttempts:  0,
	}
}

type injector struct {
	c              client.Client
	namespacedName types.NamespacedName
	retryAttempts  int
	injectorFn     InjectorFn
	cancelFn       context.CancelFunc

	l logr.Logger
}

func (r *injector) GetName() string {
	return r.namespacedName.String()
}

// Run starts the resolver. Run will not return
// before resolver loop is stopped by ctx.
func (r *injector) Run(ctx context.Context) {
	defer runtime.HandleCrash()
	// TBD do we need callbacks on start

	ctx, r.cancelFn = context.WithCancel(ctx)
	r.inject(ctx)
}

func (r *injector) Stop() {
	fmt.Println("Stop", r.GetName())
	r.cancelFn()
}

func (r *injector) inject(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.l = log.FromContext(ctx)

	wait.Until(func() {
		r.l.Info("inject loop start", "name", r.GetName())
		r.injector(ctx)
		r.l.Info("inject done", "name", r.GetName())
	}, 1*time.Minute, ctx.Done())

	r.l.Info("inject loop done", "name", r.GetName())

}

func (r *injector) injector(ctx context.Context) {
	p := backoff.NewConstantPolicy()
	b := p.Start(ctx)
	r.retryAttempts = 0

	r.l = log.FromContext(ctx)

	for backoff.Continue(b) {
		r.retryAttempts++
		r.l.Info("inject", "name", r.GetName(), "attempt", r.retryAttempts)
		if err := r.injectorFn(ctx, r.namespacedName); err != nil {
			r.l.Error(err, "injection failed")
			continue
		}
		return
	}
}
