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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/nokia/k8s-ipam/controllers/allocation"
	"github.com/nokia/k8s-ipam/controllers/injector"
	"github.com/nokia/k8s-ipam/controllers/networkinstance"
	"github.com/nokia/k8s-ipam/controllers/prefix"
	"github.com/nokia/k8s-ipam/internal/shared"
	"github.com/nokia/k8s-ipam/pkg/ipam/clientproxy"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Setup package controllers.
func Setup(ctx context.Context, mgr ctrl.Manager, opts *shared.Options) error {
	ipamproxyClient := clientproxy.New(ctx, &clientproxy.Config{
		Registrator: opts.Registrator,
	})
	opts.IpamClientProxy = ipamproxyClient

	eventChs := map[schema.GroupVersionKind]chan event.GenericEvent{}
	for _, setup := range []func(ctrl.Manager, *shared.Options) (schema.GroupVersionKind, chan event.GenericEvent, error){
		networkinstance.Setup,
		prefix.Setup,
		allocation.Setup,
	} {
		gvk, geCh, err := setup(mgr, opts)
		if err != nil {
			return err
		}
		eventChs[gvk] = geCh
	}
	for _, setup := range []func(ctrl.Manager, *shared.Options) error{
		injector.Setup,
	} {
		if err := setup(mgr, opts); err != nil {
			return err
		}
	}

	ipamproxyClient.AddEventChs(eventChs)

	return nil
}
