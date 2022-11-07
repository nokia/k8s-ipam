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
	"time"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/go-logr/logr"
	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	"github.com/henderiw-nephio/ipam/internal/injector"
	"github.com/henderiw-nephio/ipam/internal/injectors"
	"github.com/henderiw-nephio/ipam/internal/resource"
	"github.com/henderiw-nephio/ipam/internal/shared"
	"github.com/henderiw-nephio/ipam/pkg/alloc/allocpb"
	"github.com/nephio-project/nephio-controller-poc/pkg/porch"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/yaml"
)

const (
	finalizer = "ipam.nephio.org/finalizer"
	// errors
	//errGetCr        = "cannot get resource"
	//errUpdateStatus = "cannot update status"

	//reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func Setup(mgr ctrl.Manager, options *shared.Options) error {
	r := &reconciler{
		kind:        "ipam",
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		porchClient: options.PorchClient,
		allocCLient: options.AllocClient,

		injectors:    options.Injectors,
		pollInterval: options.Poll,
		finalizer:    resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&porchv1alpha1.PackageRevision{}).
		Complete(r)
}

// reconciler reconciles a NetworkInstance object
type reconciler struct {
	kind string
	client.Client
	porchClient  client.Client
	allocCLient  allocpb.AllocationClient
	Scheme       *runtime.Scheme
	injectors    injectors.Injectors
	pollInterval time.Duration
	finalizer    *resource.APIFinalizer

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile", "req", req)

	cr := &porchv1alpha1.PackageRevision{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		if resource.IgnoreNotFound(err) != nil {
			r.l.Error(err, "cannot get resource")
			return ctrl.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get resource")
		}
		return ctrl.Result{}, nil
	}

	crName := types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}
	i := injector.New(&injector.Config{
		InjectorHandler: r.injectIPs,
		NamespacedName:  crName,
		Client:          r.Client,
	})

	hasIpamReadinessGate := hasReadinessGate(cr.Spec.ReadinessGates, r.kind)
	// if no IPAM readiness gate, delete the injector if it existed or not
	// we can stop the reconciliation in this case since there is nothing more to do
	if !hasIpamReadinessGate {
		r.injectors.Stop(i)
		r.l.Info("injector stopped", "pr", cr.GetName())
		return ctrl.Result{}, nil
	}

	// run the injector when the ipam readiness gate is set
	r.l.Info("injector running", "pr", cr.GetName())
	r.injectors.Run(i)

	return ctrl.Result{}, nil
}

func hasReadinessGate(gates []porchv1alpha1.ReadinessGate, gate string) bool {
	for i := range gates {
		g := gates[i]
		if g.ConditionType == gate {
			return true
		}
	}
	return false
}

func hasCondition(conditions []porchv1alpha1.Condition, conditionType string) (*porchv1alpha1.Condition, bool) {
	for i := range conditions {
		c := conditions[i]
		if c.Type == conditionType {
			return &c, true
		}
	}
	return nil, false
}

func (r *reconciler) injectIPs(ctx context.Context, namespacedName types.NamespacedName) error {
	r.l = log.FromContext(ctx)
	r.l.Info("injector function", "name", namespacedName.String())

	origPr := &porchv1alpha1.PackageRevision{}
	if err := r.porchClient.Get(ctx, namespacedName, origPr); err != nil {
		return err
	}

	pr := origPr.DeepCopy()

	prResources := &porchv1alpha1.PackageRevisionResources{}
	if err := r.porchClient.Get(ctx, namespacedName, prResources); err != nil {
		return err
	}

	pkgBuf, err := porch.ResourcesToPackageBuffer(prResources.Spec.Resources)
	if err != nil {
		return err
	}

	for i, rn := range pkgBuf.Nodes {
		if rn.GetApiVersion() == ipamv1alpha1.GroupVersion.String() && rn.GetKind() == ipamv1alpha1.IPAllocationKind {
			ipalloc := &ipamv1alpha1.IPAllocation{}
			if err := yaml.Unmarshal([]byte(rn.MustString()), ipalloc); err != nil {
				return errors.Wrap(err, "cannot unmarchal ip allocation")
			}
			namespace := rn.GetNamespace()
			if namespace == "" {
				namespace = "default"
			}

			resp, err := r.allocCLient.Allocation(ctx, &allocpb.Request{
				Namespace: namespace,
				Name:      rn.GetName(),
				Kind:      "ipam",
				Labels:    rn.GetLabels(),
				Spec: &allocpb.Spec{
					Selector: ipalloc.Spec.Selector.MatchLabels,
				},
			})
			if err != nil {
				return errors.Wrap(err, "cannot allocate ip")
			}

			ipalloc.Status.AllocatedPrefix = resp.GetAllocatedPrefix()
			ipalloc.Status.ConditionedStatus.Conditions = append(ipalloc.Status.ConditionedStatus.Conditions, ipamv1alpha1.Condition{
				Kind:               ipamv1alpha1.ConditionKindReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			})

			b, err := yaml.Marshal(ipalloc)
			if err != nil {
				return errors.Wrap(err, "cannot marshal ip allocation")
			}

			n, err := kyaml.Parse(string(b))
			if err != nil {
				return errors.Wrap(err, "cannot parse kyaml")
			}
			// replace the node entry
			pkgBuf.Nodes[i] = n
		}
	}

	newResources, err := porch.CreateUpdatedResources(prResources.Spec.Resources, pkgBuf)
	if err != nil {
		return errors.Wrap(err, "cannot update package revision resources")
	}
	prResources.Spec.Resources = newResources
	if err = r.porchClient.Update(ctx, prResources); err != nil {
		return err
	}

	hasIpamReadinessGate := hasReadinessGate(pr.Spec.ReadinessGates, r.kind)
	ipamCondition, found := hasCondition(pr.Status.Conditions, r.kind)
	if !hasIpamReadinessGate {
		pr.Spec.ReadinessGates = append(pr.Spec.ReadinessGates, porchv1alpha1.ReadinessGate{
			ConditionType: "bar",
		})
	}

	// If the condition is not already set on the PackageRevision, set it. Otherwise just
	// make sure that the status is "True".
	if !found {
		pr.Status.Conditions = append(pr.Status.Conditions, porchv1alpha1.Condition{
			Type:   "foo",
			Status: porchv1alpha1.ConditionTrue,
		})
	} else {
		ipamCondition.Status = porchv1alpha1.ConditionTrue
	}

	// If nothing changed, then no need to update.
	// TODO: For some reason using equality.Semantic.DeepEqual and the full PackageRevision always reports a diff.
	// We should find out why.
	if equality.Semantic.DeepEqual(origPr.Spec.ReadinessGates, pr.Spec.ReadinessGates) &&
		equality.Semantic.DeepEqual(origPr.Status, pr.Status) {
		return nil
	}

	if err := r.Update(ctx, pr); err != nil {
		return errors.Wrap(err, "cannot update packagerevision")
	}

	return nil
}
