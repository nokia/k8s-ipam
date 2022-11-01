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
	"fmt"
	"strconv"
	"strings"
	"time"

	"inet.af/netaddr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"github.com/hansthienpondt/goipam/pkg/table"
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/ipam"
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
	}

	niHandler := &EnqueueRequestForAllNetworkInstances{
		client: mgr.GetClient(),
		ctx:    context.Background(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPPrefix{}).
		Watches(&source.Kind{Type: &ipamv1alpha1.NetworkInstance{}}, niHandler).
		Complete(r)
}

// reconciler reconciles a IPPrefix object
type reconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Ipam         ipam.Ipam
	pollInterval time.Duration
	finalizer    *resource.APIFinalizer

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
			return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get resource")
		}
		return reconcile.Result{}, nil
	}

	if meta.WasDeleted(cr) {
		allocs, err := r.buildIPAllocation(cr)
		if err != nil {
			r.l.Error(err, "cannot build alloc")
			cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Failed(err.Error()))
			return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}

		// since we deallocate based on the allocation-name we can just use the first alloc from allocs
		// to delete the prefix
		if err := r.Ipam.DeAllocateIPPrefix(ctx, allocs[0]); err != nil {
			if !(strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "not found")) {
				r.l.Error(err, "cannot delete resource")
				cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Unknown())
				return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
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
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      cr.Spec.NetworkInstance,
	}, ni); err != nil {
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

	allocs, err := r.buildIPAllocation(cr)
	if err != nil {
		r.l.Error(err, "cannot build alloc")
		cr.SetConditions(ipamv1alpha1.ReconcileError(err), ipamv1alpha1.Failed(err.Error()))
		return reconcile.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}

	// to deal with change we check the existing prefix against the status
	// since we deallocate based on the allocation-name we can just use the first alloc from allocs
	// to delete the prefix
	if cr.Status.Prefix != cr.Spec.Prefix {
		if len(allocs) > 0 {
			if err := r.Ipam.DeAllocateIPPrefix(ctx, allocs[0]); err != nil {
				r.l.Info("cannot deallocate prefix", "err", err)
				cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed(err.Error()))
				return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
	}

	// allocate prefixes
	if len(allocs) == 0 {
		cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed("no parent prefix found"))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	for _, alloc := range allocs {
		_, allocatedPrefix, err := r.Ipam.AllocateIPPrefix(ctx, alloc)
		if err != nil {
			r.l.Info("cannot allocate prefix", "err", err)
			cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Failed(err.Error()))
			return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
		if *allocatedPrefix != alloc.Spec.Prefix {
			// we got a different prefix than requested
			r.l.Error(err, "prefix allocation failed", "requested", alloc.Spec.Prefix, "allocated", *allocatedPrefix)
			cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Unknown())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
		}
	}
	r.l.Info("Successfully reconciled resource")
	cr.Status.Prefix = cr.Spec.Prefix
	cr.SetConditions(ipamv1alpha1.ReconcileSuccess(), ipamv1alpha1.Ready())
	return ctrl.Result{}, errors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) buildIPAllocation(cr *ipamv1alpha1.IPPrefix) ([]*ipamv1alpha1.IPAllocation, error) {
	p, err := netaddr.ParseIPPrefix(cr.Spec.Prefix)
	if err != nil {
		r.l.Error(err, "cannot parse prefix")
		return nil, err
	}

	var af string
	if p.IP().Is4() {
		af = string(ipamv1alpha1.AddressFamilyIpv4)
	}
	if p.IP().Is6() {
		af = string(ipamv1alpha1.AddressFamilyIpv6)
	}

	prefixSize, _ := p.IPNet().Mask.Size()
	prefixLength := strconv.Itoa(prefixSize)

	// augment labels with additional metadata
	labels := cr.GetLabels()
	if len(labels) == 0 {
		labels = map[string]string{}
	}
	// add address family to the labels associated to the prefix
	labels[ipamv1alpha1.NephioAddressFamilyKey] = af
	// add the prefix length to the labels associated to the prefix
	labels[ipamv1alpha1.NephioPrefixLengthKey] = prefixLength
	// add prefix name to the labels associated to the prefix
	labels[ipamv1alpha1.NephioIPPrefixNameKey] = cr.GetName()
	// add prefix name to the labels associated to the prefix
	labels[ipamv1alpha1.NephioOriginKey] = string(ipamv1alpha1.OriginIPPrefix)
	// add ip pool labelKey if present
	if cr.Spec.Pool {
		labels[ipamv1alpha1.NephioPoolKey] = strconv.FormatBool(cr.Spec.Pool)
	}

	// labels used to select the prefix context
	selectorLabels := map[string]string{
		ipamv1alpha1.NephioNetworkInstanceKey: cr.Spec.NetworkInstance,
	}

	allocs := []*ipamv1alpha1.IPAllocation{}
	// if we have a have an address in the prefix we add an additional prefix (representing the address in the ipam)
	if p.IPNet().String() != p.Masked().String() {
		allocs = append(allocs, &ipamv1alpha1.IPAllocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cr.GetNamespace(),
				Name:      cr.GetName(),
				Labels:    labels,
			},
			Spec: ipamv1alpha1.IPAllocationSpec{
				Prefix: p.Masked().String(),
				Selector: &metav1.LabelSelector{
					MatchLabels: selectorLabels,
				},
			},
		})

		var addressPrefixLength string
		if af == string(ipamv1alpha1.AddressFamilyIpv4) {
			addressPrefixLength = "32"
		} else {
			addressPrefixLength = "128"
		}
		address := strings.Join([]string{p.IP().String(), addressPrefixLength}, "/")

		// copy labels
		gatewayLabels := map[string]string{}
		for k, v := range labels {
			gatewayLabels[k] = v
		}
		gatewayLabels[ipamv1alpha1.NephioGatewayKey] = "true"
		gatewayLabels[ipamv1alpha1.NephioPrefixLengthKey] = prefixLength
		n := strings.Split(p.Masked().IPNet().String(), "/")
		gatewayLabels[ipamv1alpha1.NephioParentNetKey] = n[0]
		gatewayLabels[ipamv1alpha1.NephioParentPrefixLengthKey] = prefixLength

		allocs = append(allocs, &ipamv1alpha1.IPAllocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cr.GetNamespace(),
				Name:      cr.GetName(),
				Labels:    gatewayLabels,
			},
			Spec: ipamv1alpha1.IPAllocationSpec{
				Prefix: address,
				Selector: &metav1.LabelSelector{
					MatchLabels: selectorLabels,
				},
			},
		})
	} else {
		// when the address length is /32 or /128 we want to use the prefix-name of the
		// higher order prefix
		if (af == string(ipamv1alpha1.AddressFamilyIpv4) && (prefixLength == "32")) ||
			(af == string(ipamv1alpha1.AddressFamilyIpv6) && (prefixLength == "128")) {
			niName := types.NamespacedName{
				Namespace: cr.GetNamespace(),
				Name:      cr.Spec.NetworkInstance,
			}
			rt, ok := r.Ipam.Get(niName.String())
			if !ok {
				return nil, fmt.Errorf("ipam ni not ready or network-instance %s not correct", cr.Spec.NetworkInstance)
			}
			psize := 0
			var adjRoute *table.Route
			for _, route := range rt.Parents(p) {
				r.l.Info("address", "route", route)
				p := route.IPPrefix()
				ps, _ := p.IPNet().Mask.Size()
				if ps > psize {
					psize = ps
					adjRoute = route
				}
			}
			if adjRoute != nil {
				r.l.Info("selected adj prefix for address", "route", adjRoute)
				addressLabels := map[string]string{}
				for k, v := range labels {
					addressLabels[k] = v
				}
				addressLabels[ipamv1alpha1.NephioIPPrefixNameKey] = adjRoute.GetLabels().Get(ipamv1alpha1.NephioIPPrefixNameKey)

				p := netaddr.MustParseIPPrefix(adjRoute.String())
				prefixSize, _ := p.IPNet().Mask.Size()
				prefixLength := strconv.Itoa(prefixSize)

				n := strings.Split(p.Masked().IPNet().String(), "/")
				addressLabels[ipamv1alpha1.NephioParentNetKey] = n[0]
				addressLabels[ipamv1alpha1.NephioParentPrefixLengthKey] = prefixLength

				allocs = append(allocs, &ipamv1alpha1.IPAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cr.GetNamespace(),
						Name:      cr.GetName(),
						Labels:    addressLabels,
					},
					Spec: ipamv1alpha1.IPAllocationSpec{
						Prefix: cr.Spec.Prefix,
						Selector: &metav1.LabelSelector{
							MatchLabels: selectorLabels,
						},
					},
				})
			}

		} else {
			allocs = append(allocs, &ipamv1alpha1.IPAllocation{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cr.GetNamespace(),
					Name:      cr.GetName(),
					Labels:    labels,
				},
				Spec: ipamv1alpha1.IPAllocationSpec{
					Prefix: cr.Spec.Prefix,
					Selector: &metav1.LabelSelector{
						MatchLabels: selectorLabels,
					},
				},
			})
		}
	}

	return allocs, nil
}
