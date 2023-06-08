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

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/nipam/pkg/table"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Getter interface {
	GetIPClaim(ctx context.Context) error
}

type GetterConfig struct {
	claim *ipamv1alpha1.IPClaim
	rib   *table.RIB
}

func NewGetter(c *GetterConfig) Getter {
	return &getter{
		claim: c.claim,
		rib:   c.rib,
	}
}

type getter struct {
	claim *ipamv1alpha1.IPClaim
	rib   *table.RIB
	l     logr.Logger
}

func (r *getter) GetIPClaim(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetName(), "kind", r.claim.Spec.Kind)
	r.l.Info("dynamic claim")

	labelSelector, err := r.claim.GetLabelSelector()
	if err != nil {
		return err
	}
	routes := r.rib.GetByLabel(labelSelector)
	if len(routes) != 0 {
		// update the status
		r.claim.Status.Prefix = pointer.String(routes[0].Prefix().String())
		if r.claim.Spec.Kind == ipamv1alpha1.PrefixKindNetwork {
			if r.claim.Spec.CreatePrefix == nil {
				r.claim.Status.Gateway = pointer.String(r.getGateway(*r.claim.Status.Prefix))
			}
		}
	}
	return nil
}

func (r *getter) getGateway(prefix string) string {
	pi, err := iputil.New(prefix)
	if err != nil {
		r.l.Error(err, "cannot get gateway parent rpefix")
		return ""
	}
	gatewaySelector, err := r.claim.GetGatewayLabelSelector(string(pi.GetSubnetName()))
	if err != nil {
		r.l.Error(err, "cannot get gateway label selector")
		return ""
	}
	r.l.Info("gateway", "gatewaySelector", gatewaySelector)
	routes := r.rib.GetByLabel(gatewaySelector)
	if len(routes) > 0 {
		r.l.Info("gateway", "routes", routes)
		return routes[0].Prefix().Addr().String()
	}
	return ""
}
