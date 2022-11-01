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

	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/pkg/alloc/allocpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (s *subServer) AllocationRequest(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	s.l = log.FromContext(ctx)
	s.l.Info("allocate", "alloc", alloc)

	parentPrefix, prefix, err := s.ipam.AllocateIPPrefix(ctx, buildAlloc(alloc))
	if err != nil {
		return nil, err
	}
	return &allocpb.Response{
		Prefix:       *prefix,
		ParentPrefix: *parentPrefix,
	}, nil
}

func (s *subServer) DeAllocationRequest(ctx context.Context, alloc *allocpb.Request) (*allocpb.Response, error) {
	s.l = log.FromContext(ctx)
	s.l.Info("deallocate", "alloc", alloc)

	if err := s.ipam.DeAllocateIPPrefix(ctx, buildAlloc(alloc)); err != nil {
		return nil, err
	}
	return &allocpb.Response{}, nil
}

func buildAlloc(alloc *allocpb.Request) *ipamv1alpha1.IPAllocation {
	return &ipamv1alpha1.IPAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: alloc.Namespace,
			Name:      alloc.Name,
			Labels:    alloc.Labels,
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			Prefix:       alloc.GetSpec().GetPrefix(),
			PrefixLength: uint8(alloc.GetSpec().GetPrefixLength()),
			Selector: &metav1.LabelSelector{
				MatchLabels: alloc.GetSpec().GetSelector(),
			},
		},
	}
}
