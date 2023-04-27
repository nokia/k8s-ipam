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

package shared

import (
	"time"

	"github.com/henderiw-k8s-lcnc/discovery/registrator"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

type Options struct {
	PorchClient     client.Client
	Registrator     registrator.Registrator
	IpamClientProxy clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]
	VlanClientProxy clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation]
	Poll            time.Duration
	Copts           controller.Options
	Ipam            backend.Backend
	Vlan            backend.Backend
}
