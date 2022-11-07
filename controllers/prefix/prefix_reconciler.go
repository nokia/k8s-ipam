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

package prefix

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/allocator"
	"github.com/henderiw-nephio/ipam/internal/ipam2"
	"github.com/henderiw-nephio/ipam/internal/meta"
	"github.com/henderiw-nephio/ipam/internal/resource"
	"github.com/henderiw-nephio/ipam/internal/shared"
	"github.com/pkg/errors"
)

const (
	finalizer = "ipam.nephio.org/finalizer"
	// error
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=ipam.nephio.org,resources=ipprefixes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=ipprefixes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.nephio.org,resources=ipprefixes/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func Setup(mgr ctrl.Manager, options *shared.Options) error {
	r := &reconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Ipam:         options.Ipam,
		pollInterval: options.Poll,
		finalizer:    resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
		//prefixValidators: initializePrefixValidators(options.Ipam),
		allocator: allocator.New(),
	}

	/*
		niHandler := &EnqueueRequestForAllNetworkInstances{
			client: mgr.GetClient(),
			ctx:    context.Background(),
		}
	*/

	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPPrefix{}).
		//Watches(&source.Kind{Type: &ipamv1alpha1.NetworkInstance{}}, niHandler).
		Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Ipam         ipam2.Ipam
	pollInterval time.Duration
	finalizer    *resource.APIFinalizer
	//prefixValidators PrefixValidators
	allocator allocator.Allocator

	l logr.Logger
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IPPrefix object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &ipamv1alpha1.IPPrefix{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, "cannot get resource")
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get resource")
		}
		return reconcile.Result{}, nil
	}
	niName := types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      cr.Spec.NetworkInstance,
	}

	if meta.WasDeleted(cr) {
		// if the prefix condition is false it means the prefix was not supplied to the network
		// we can delete it w/o deleting it from the IPAM
		if cr.GetCondition(ipamv1alpha1.ConditionKindReady).Status == corev1.ConditionTrue {
			if err := r.Ipam.DeAllocateIPPrefix(ctx, ipam2.BuildAllocationFromIPPrefix(cr)); err != nil {
				if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
					r.l.Error(err, "cannot delete resource")
					cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
					return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
				}
			}
			/*
				// if rt does not exist we can delete immediately since the ipam table is clean
				if _, ok := r.Ipam.Get(niName.String()); ok {
					allocs := r.allocator.Allocate(cr)

					// for a network we need to delete all remaining allocation if this is the latest prefix in the network
					if cr.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindNetwork) &&
						r.Ipam.IsLatestPrefixInNetwork(cr) {
						// delete all prefixes
						if err := r.Ipam.DeAllocateIPPrefixes(ctx, allocs...); err != nil {
							if !(strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "not found")) {
								r.l.Error(err, "cannot delete resource")
								cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
								return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
							}
						}

					} else {
						// since we deallocate based on the allocation-name we can just use the first alloc from allocs
						// to delete the prefix
						if err := r.Ipam.DeAllocateIPPrefixes(ctx, allocs[0]); err != nil {
							if !(strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "not found")) {
								r.l.Error(err, "cannot delete resource")
								cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
								return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
							}
						}
					}

				}
			*/
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		r.l.Info("Successfully deleted resource")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		r.l.Error(err, "cannot add finalizer")
		cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// this block is here to deal with network instance deletion
	// we ensure the condition is set to false if the networkinstance is deleted
	ni := &ipamv1alpha1.NetworkInstance{}
	if err := r.Get(ctx, niName, ni); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		r.l.Info("cannot allocate prefix, network-intance not found")
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("network-instance not found"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// check deletion timestamp
	if meta.WasDeleted(ni) {
		r.l.Info("cannot allocate prefix, network-intance not ready")
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("network-instance not ready"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	/*
		msg, err := r.validatePrefix(ctx, cr)
		if err != nil {
			r.l.Error(err, "cannot validate prefix")
			cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Failed(err.Error()))
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		if msg != "" {
			r.l.Info("prefix validation failed", "msg", msg)
			cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed(msg))
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
	*/

	// The spec got changed we check the existing prefix against the status
	if (cr.Status.AllocatedPrefix != "" && cr.Status.AllocatedPrefix != cr.Spec.Prefix) ||
		(cr.Status.AllocatedNetwork != "" && cr.Status.AllocatedNetwork != cr.Spec.Network) {
		if err := r.Ipam.DeAllocateIPPrefix(ctx, ipam2.BuildAllocationFromIPPrefix(cr)); err != nil {
			if !strings.Contains(err.Error(), "not ready") || !strings.Contains(err.Error(), "not found") {
				r.l.Error(err, "cannot delete resource")
				cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
				return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		}

		/*
				if len(allocs) > 0 {
				// for a network we need to delete all remaining allocation if this is the latest prefix in the network
				if cr.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindNetwork) &&
					r.Ipam.IsLatestPrefixInNetwork(cr) {
					// delete all prefixes
					if err := r.Ipam.DeAllocateIPPrefixes(ctx, allocs...); err != nil {
						if !(strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "not found")) {
							r.l.Error(err, "cannot delete resource")
							cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
							return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
						}
					}
				} else {
					// since we deallocate based on the allocation-name we can just use the first alloc from allocs
					// to delete the prefix
					if err := r.Ipam.DeAllocateIPPrefixes(ctx, allocs[0]); err != nil {
						if !(strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "not found")) {
							r.l.Error(err, "cannot delete resource")
							cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
							return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
						}
					}
				}
			}
		*/
	}

	// allocate prefixes
	/*
		if len(allocs) == 0 {
			cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("no parent prefix found"))
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		for _, alloc := range allocs {
			_, err := r.Ipam.AllocateIPPrefix(ctx, alloc, false, nil)
			if err != nil {
				r.l.Info("cannot allocate prefix", "err", err)
				cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed(err.Error()))
				return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}

			//	if *allocatedPrefix != alloc.Spec.Prefix {
					// we got a different prefix than requested
			//		r.l.Error(err, "prefix allocation failed", "requested", alloc.Spec.Prefix, "allocated", *allocatedPrefix)
			//		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Unknown())
			//		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			//	}
		}
	*/
	allocatedPrefix, err := r.Ipam.AllocateIPPrefix(ctx, ipam2.BuildAllocationFromIPPrefix(cr))
	if err != nil {
		r.l.Info("cannot allocate prefix", "err", err)
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed(err.Error()))
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	if allocatedPrefix.AllocatedPrefix != cr.Spec.Prefix {
		//we got a different prefix than requested
		r.l.Error(err, "prefix allocation failed", "requested", cr.Spec.Prefix, "allocated", *allocatedPrefix)
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Unknown())
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	r.l.Info("Successfully reconciled resource")
	cr.Status.AllocatedPrefix = cr.Spec.Prefix
	// only relevant for prefixkind network but does not harm
	cr.Status.AllocatedNetwork = cr.Spec.Network
	cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

/*
func (r *reconciler) validatePrefix(ctx context.Context, cr *ipamv1alpha1.IPPrefix) (msg string, err error) {
	p, err := netaddr.ParseIPPrefix(cr.Spec.Prefix)
	if err != nil {
		return "", err
	}
	v := r.prefixValidators.Get(ipamv1alpha1.PrefixKind(cr.Spec.PrefixKind))
	if v == nil {
		return "", fmt.Errorf("wrong prefix kind: %s", cr.Spec.PrefixKind)
	}

	switch cr.Spec.PrefixKind {
	case string(ipamv1alpha1.PrefixKindNetwork):
		msg, err = v.Validate(ctx, cr, p.Masked())
	default:
		msg, err = v.Validate(ctx, cr, p)
	}
	if err != nil {
		return "", err
	}
	return msg, nil

}
*/

/*
func (r *reconciler) buildIPAllocation(cr *ipamv1alpha1.IPPrefix) ([]*ipamv1alpha1.IPAllocation, error) {
	// since validation passed we can assume the parsing will succeed
	p := netaddr.MustParseIPPrefix(cr.Spec.Prefix)

	// labels set in meta data
	labels := getLabels(cr, p)
	// labels used to select the prefix context
	selectorLabels := getSelectorLabels(cr)

	allocs := []*ipamv1alpha1.IPAllocation{}

	if cr.Spec.PrefixKind == string(ipamv1alpha1.PrefixKindNetwork) {
		// for a network based prefix we have 2 options
		// a prefix where the address part of the net is equal to the net and another one where it is not
		if p.IPNet().String() != p.Masked().String() {
			allocs = append(allocs, buildNetAllocation(cr, p.Masked(), getNetLabels(labels, p), selectorLabels))

			allocs = append(allocs, buildFirstAddressAllocation(cr, p.Masked(), getFirstLabels(labels, p), selectorLabels))
			// TODO do we also need to create last address
			allocs = append(allocs, buildAddressAllocation(cr, p, getAddressLabels(labels, p), selectorLabels))
		} else {
			allocs = append(allocs, buildNetAllocation(cr, p.Masked(), getNetLabels(labels, p), selectorLabels))
			allocs = append(allocs, buildAddressAllocation(cr, p, getAddressLabels(labels, p), selectorLabels))
		}
	} else {
		allocs = append(allocs, buildAllocation(cr, p, labels, selectorLabels))
	}

	return allocs, nil
}
*/

/*
func buildNetAllocation(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix, labels, selectorLabels map[string]string) *ipamv1alpha1.IPAllocation {
	labels[ipamv1alpha1.NephioIPPrefixNameKey] = cr.GetName()
	labels[ipamv1alpha1.NephioPrefixKind] = string(ipamv1alpha1.PrefixKindNetwork)
	return &ipamv1alpha1.IPAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cr.GetNamespace(),
			Name:      strings.Join([]string{p.IP().String(), iputil.GetPrefixLength(p)}, "-"),
			Labels:    labels,
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			Prefix: p.String(),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}
}
*/

/*
func buildAllocation(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix, labels, selectorLabels map[string]string) *ipamv1alpha1.IPAllocation {
	return &ipamv1alpha1.IPAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cr.GetNamespace(),
			Name:      cr.GetName(),
			Labels:    labels,
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			Prefix: p.String(),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}
}
*/

/*
func buildAddressAllocation(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix, labels, selectorLabels map[string]string) *ipamv1alpha1.IPAllocation {
	labels[ipamv1alpha1.NephioPrefixKind] = string(ipamv1alpha1.PrefixKindNetwork)
	return &ipamv1alpha1.IPAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cr.GetNamespace(),
			Name:      cr.GetName(),
			Labels:    labels,
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			Prefix: iputil.GetAddress(p),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}
}
*/

/*
func buildFirstAddressAllocation(cr *ipamv1alpha1.IPPrefix, p netaddr.IPPrefix, labels, selectorLabels map[string]string) *ipamv1alpha1.IPAllocation {
	labels[ipamv1alpha1.NephioPrefixKind] = string(ipamv1alpha1.PrefixKindNetwork)
	return &ipamv1alpha1.IPAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cr.GetNamespace(),
			Name:      "first",
			Labels:    labels,
		},
		Spec: ipamv1alpha1.IPAllocationSpec{
			Prefix: iputil.GetFirstAddress(p),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}
}
*/

/*
func getNetLabels(l map[string]string, p netaddr.IPPrefix) map[string]string {
	// copy labels
	labels := map[string]string{}
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(p))
	labels[ipamv1alpha1.NephioPrefixLengthKey] = iputil.GetPrefixLength(p)
	labels[ipamv1alpha1.NephioOriginKey] = string(ipamv1alpha1.OriginIPSystem)
	labels[ipamv1alpha1.NephioIPPrefixNameKey] = "net"
	return labels
}
*/

/*
func getAddressLabels(l map[string]string, p netaddr.IPPrefix) map[string]string {
	labels := map[string]string{}
	for k, v := range labels {
		labels[k] = v
	}
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(p))
	labels[ipamv1alpha1.NephioPrefixLengthKey] = iputil.GetAddressPrefixLength(p)
	labels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.GetPrefixLength(p)
	return labels
}
*/

/*
func getFirstLabels(l map[string]string, p netaddr.IPPrefix) map[string]string {
	// copy labels
	labels := map[string]string{}
	for k, v := range l {
		labels[k] = v
	}
	// set gateway to true
	labels[ipamv1alpha1.NephioIPPrefixNameKey] = "first"
	labels[ipamv1alpha1.NephioAddressFamilyKey] = string(iputil.GetAddressFamily(p))
	labels[ipamv1alpha1.NephioOriginKey] = string(ipamv1alpha1.OriginIPSystem)
	labels[ipamv1alpha1.NephioPrefixLengthKey] = iputil.GetAddressPrefixLength(p)
	labels[ipamv1alpha1.NephioParentPrefixLengthKey] = iputil.GetPrefixLength(p)
	return labels
}
*/
