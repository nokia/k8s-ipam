/*
 Copyright 2023 The Nephio Authors.

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

package lease

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/nephio-project/nephio/controllers/pkg/resource"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultLeaseInterval = 1 * time.Second
)

type Lease interface {
	AcquireLease(ctx context.Context, cr client.Object) error
}

func New(c client.Client, leaseNSN types.NamespacedName) Lease {
	return &lease{
		Client:         c,
		leasName:       leaseNSN.Name,
		leaseNamespace: leaseNSN.Namespace,
	}
}

type lease struct {
	client.Client

	leasName       string
	leaseNamespace string

	l logr.Logger
}

func getHolderIdentity(cr client.Object) string {
	return fmt.Sprintf("%s.%s.%s",
		strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind),
		strings.ToLower(cr.GetNamespace()),
		strings.ToLower(cr.GetName()))
}

func (r *lease) getLease(cr client.Object) *coordinationv1.Lease {
	now := metav1.NowMicro()
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.leasName,
			Namespace: r.leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       pointer.String(getHolderIdentity(cr)),
			LeaseDurationSeconds: pointer.Int32(int32(defaultLeaseInterval / time.Second)),
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}
}

func (r *lease) AcquireLease(ctx context.Context, cr client.Object) error {
	r.l = log.FromContext(ctx)
	r.l.Info("attempting to acquire lease to update the resource", "lease", r.leasName)
	interconnectLeaseNSN := types.NamespacedName{
		Name:      r.leasName,
		Namespace: r.leaseNamespace,
	}

	lease := &coordinationv1.Lease{}
	if err := r.Get(ctx, interconnectLeaseNSN, lease); err != nil {
		if resource.IgnoreNotFound(err) != nil {
			return err
		}
		r.l.Info("lease not found, creating it", "lease", r.leasName)

		lease = r.getLease(cr)
		if err := r.Create(ctx, lease); err != nil {
			return err
		}
	}
	// get the lease again
	if err := r.Get(ctx, interconnectLeaseNSN, lease); err != nil {
		return err
	}

	if lease == nil || lease.Spec.HolderIdentity == nil {
		return fmt.Errorf("lease nil or holderidentity nil")
	}

	now := metav1.NowMicro()
	if *lease.Spec.HolderIdentity != getHolderIdentity(cr) {
		// lease is held by another
		r.l.Info("lease held by another identity", "identity", *lease.Spec.HolderIdentity)
		if lease.Spec.RenewTime != nil {
			expectedRenewTime := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
			if !expectedRenewTime.Before(now.Time) {
				r.l.Info("cannot acquire lease, lease held by another identity", "identity", *lease.Spec.HolderIdentity)
				return fmt.Errorf("cannot acquire lease, lease held by another identity: %s", *lease.Spec.HolderIdentity)
			}
		}
	}

	// take over the lease or update the lease
	r.l.Info("successfully acquired lease")
	newLease := r.getLease(cr)
	newLease.SetResourceVersion(lease.ResourceVersion)
	if err := r.Update(ctx, newLease); err != nil {
		return err
	}
	return nil
}
