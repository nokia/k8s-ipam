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

	"github.com/hansthienpondt/nipam/pkg/table"
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/nokia/k8s-ipam/pkg/proto/resourcepb"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Delete deletes the claimation based on the ownerslector and deletes all prefixes associated with the ownerseelctor
// if no prefixes are found, no error is returned
func (r *applicator) Delete(ctx context.Context) error {
	r.l = log.FromContext(ctx).WithValues("name", r.claim.GetGenericNamespacedName(), "kind", r.claim.Spec.Kind)
	r.l.Info("delete")

	ownerSelector, err := r.claim.GetOwnerSelector()
	if err != nil {
		return err
	}
	r.l.Info("delete claim individual prefix", "nsnSelector", ownerSelector)

	routes := r.rib.GetByLabel(ownerSelector)
	for _, route := range routes {
		r.l = log.FromContext(ctx).WithValues("route prefix", route.Prefix())

		// this is a delete
		// only update when not initializing
		// only update when the prefix is a non /32 or /128
		// only update when the parent is a create prefix type
		if !r.initializing && !iputil.NewPrefixInfo(route.Prefix()).IsAddressPrefix() && (r.claim.Spec.CreatePrefix != nil) {
			r.l.Info("route exists", "handle update for route", route, "labels", r.claim.GetFullLabels())
			// delete the children from the rib
			// update the once that have a nsn different from the origin
			childRoutesToBeUpdated := []table.Route{}
			for _, childRoute := range route.Children(r.rib) {
				r.l.Info("route exists", "handle update for route", route, "child route", childRoute)
				if childRoute.Labels()[resourcev1alpha1.NephioNsnNameKey] != r.claim.GetFullLabels()[resourcev1alpha1.NephioNsnNameKey] ||
					childRoute.Labels()[resourcev1alpha1.NephioNsnNamespaceKey] != r.claim.GetFullLabels()[resourcev1alpha1.NephioNsnNamespaceKey] {
					childRoutesToBeUpdated = append(childRoutesToBeUpdated, childRoute)
					if err := r.rib.Delete(childRoute); err != nil {
						r.l.Error(err, "cannot delete route from rib", "route", childRoute)
					}
				}
			}
			// handler watch update to the source owner controller
			r.l.Info("route exists", "handle update for route", route, "child routes", childRoutesToBeUpdated)
			r.watcher.handleUpdate(ctx, childRoutesToBeUpdated, resourcepb.StatusCode_Unknown)
		}

		if err := r.rib.Delete(route); err != nil {
			return err
		}
		if !r.initializing {
			// handler watch update to the source owner controller
			r.watcher.handleUpdate(ctx, route.Children(r.rib), resourcepb.StatusCode_Unknown)
		}
	}
	return nil
}
