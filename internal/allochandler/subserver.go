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

package allochandler

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/nokia/k8s-ipam/internal/ipam"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
)

type Options struct {
	Ipam ipam.Ipam
}

type SubServer interface {
	Allocation(context.Context, *allocpb.Request) (*allocpb.Response, error)
	DeAllocation(context.Context, *allocpb.Request) (*allocpb.Response, error)
}

func New(o *Options) SubServer {
	s := &subServer{
		ipam: o.Ipam,
	}
	return s
}

type subServer struct {
	l    logr.Logger
	ipam ipam.Ipam
}
