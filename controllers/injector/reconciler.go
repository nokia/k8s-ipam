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
	"fmt"
	"strings"
	"time"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	kptfile "github.com/GoogleContainerTools/kpt/pkg/api/kptfile/v1"
	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/henderiw-nephio/nf-injector-controller/pkg/ipam"
	"github.com/nephio-project/nephio-controller-poc/pkg/porch"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/injectors"
	"github.com/nokia/k8s-ipam/internal/resource"
	"github.com/nokia/k8s-ipam/internal/shared"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	finalizer         = "ipam.nephio.org/finalizer"
	ipamConditionType = "ipam.nephio.org.IPAMAllocation"
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

	// we just check for ipam conditions and we dont care if it is satisfied already
	// this allows us to refresh the ipam allocation status
	ipamConditions := unsatisfiedConditions(cr.Status.Conditions, ipamConditionType)

	if len(ipamConditions) > 0 {
		crName := types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      cr.Name,
		}

		r.l.Info("injector running", "pr", cr.GetName())
		if err := r.injectIPs(ctx, crName); err != nil {
			r.l.Error(err, "injection error")
			return ctrl.Result{}, err
		}
	}

	/*
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
	*/

	return ctrl.Result{}, nil
}

func unsatisfiedConditions(conditions []porchv1alpha1.Condition, conditionType string) []porchv1alpha1.Condition {
	var uc []porchv1alpha1.Condition
	for _, c := range conditions {

		// TODO: make this smarter
		// for now, just check if it is True. It means we won't re-inject if some input changes,
		// unless someone flips the state
		/*
			if c.Status != porchv1alpha1.ConditionTrue && strings.HasPrefix(c.Type, conditionType+".") {
				uc = append(uc, c)
			}
		*/
		// TBD: for now it is good to refresh the ipam status since it allows to refresh the status of the ipam
		if strings.HasPrefix(c.Type, conditionType+".") {
			uc = append(uc, c)
		}
	}

	return uc
}

/*
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
*/

func (r *reconciler) injectIPs(ctx context.Context, namespacedName types.NamespacedName) error {
	r.l = log.FromContext(ctx)
	r.l.Info("injector function", "name", namespacedName.String())

	origPr := &porchv1alpha1.PackageRevision{}
	if err := r.porchClient.Get(ctx, namespacedName, origPr); err != nil {
		return err
	}

	pr := origPr.DeepCopy()

	r.l.Info("injector function", "name", namespacedName.String(), "pr spec", pr.Spec)

	prConditions := convertConditions(pr.Status.Conditions)

	prResources, pkgBuf, err := r.injectAllocatedIPs(ctx, namespacedName, prConditions, pr)
	if err != nil {
		r.l.Error(err, "error allocating or injecting IP(s)")
		if pkgBuf == nil {
			return err
		}
		// for now just assume the error applies to all IP injections
		for _, c := range *prConditions {
			if !strings.HasPrefix(c.Type, ipamConditionType) {
				continue
			}
			if meta.IsStatusConditionTrue(*prConditions, c.Type) {
				continue
			}
			meta.SetStatusCondition(prConditions, metav1.Condition{Type: c.Type, Status: metav1.ConditionFalse,
				Reason: "ErrorDuringInjection", Message: err.Error()})
		}
	}

	pr.Status.Conditions = unconvertConditions(prConditions)

	// conditions are stored in the Kptfile right now
	for i, n := range pkgBuf.Nodes {
		if n.GetKind() == "Kptfile" {
			// we need to update the status
			nStr := n.MustString()
			var kf kptfile.KptFile
			if err := kyaml.Unmarshal([]byte(nStr), &kf); err != nil {
				return err
			}
			if kf.Status == nil {
				kf.Status = &kptfile.Status{}
			}
			kf.Status.Conditions = conditionsToKptfile(prConditions)

			kfBytes, _ := kyaml.Marshal(kf)
			node := kyaml.MustParse(string(kfBytes))
			pkgBuf.Nodes[i] = node
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

	return nil
}

func (r *reconciler) injectAllocatedIPs(ctx context.Context, namespacedName types.NamespacedName,
	prConditions *[]metav1.Condition,
	pr *porchv1alpha1.PackageRevision) (*porchv1alpha1.PackageRevisionResources, *kio.PackageBuffer, error) {

	prResources := &porchv1alpha1.PackageRevisionResources{}
	if err := r.porchClient.Get(ctx, namespacedName, prResources); err != nil {
		return nil, nil, err
	}

	for res, resdata := range prResources.Spec.Resources {
		r.l.Info("inject and allocate IP(s)", "resource", res, "data", resdata)
	}

	// need to fix back to when the PR is fixed
	pkgBuf, err := ResourcesToPackageBuffer(prResources.Spec.Resources)
	if err != nil {
		return prResources, nil, err
	}

	for i, rn := range pkgBuf.Nodes {
		r.l.Info("resource", "apiVersion", rn.GetApiVersion(), "kind", rn.GetKind())
		if rn.GetApiVersion() == "ipam.nephio.org/v1alpha1" && rn.GetKind() == "IPAllocation" {

			namespace := "default"
			if rn.GetNamespace() != "" {
				namespace = rn.GetNamespace()
			}

			// convert the yaml string to a typed IP allocation
			ipAllocSpec, err := getIpAllocationSpec(rn)
			if err != nil {
				return prResources, pkgBuf, fmt.Errorf("cannot convert ip allocation to a typed spec: %s", rn.GetName())
			}

			// get the grpc format for the allocation
			grpcAllocSpec, err := getGrpcAllocationSpec(ipAllocSpec)
			if err != nil {
				return prResources, pkgBuf, err
			}
			r.l.Info("grpc ipam allocation request", "Name", rn.GetName(), "Labels", rn.GetLabels(), "Spec", grpcAllocSpec)

			// grpc allocation request
			// we always refresh the ipallocation even if it was already satisfied,
			//since this allows to refresh the ipam
			resp, err := r.allocCLient.Allocation(ctx, &allocpb.Request{
				Namespace: namespace,
				Name:      rn.GetName(),
				Kind:      "ipam",
				Labels:    rn.GetLabels(),
				Spec:      grpcAllocSpec,
			})
			if err != nil {
				r.l.Error(err, "grpc ipam allocation request error")
				return prResources, pkgBuf, errors.Wrap(err, "cannot allocate ip")
			}

			r.l.Info("grpc ipam allocation response", "Name", rn.GetName(), "resp", resp)

			// update Allocation
			ipAllocation, err := GetUpdatedAllocation(resp, ipamv1alpha1.PrefixKind(ipAllocSpec.PrefixKind))
			if err != nil {
				return prResources, pkgBuf, errors.Wrap(err, "cannot get updated allocation status")
			}

			// update only the status in the allocation
			n := pkgBuf.Nodes[i]
			conditionType := fmt.Sprintf("%s.%s.%s.Injected", ipamConditionType, rn.GetName(), namespace)
			field := ipAllocation.Field("status")
			if err := n.SetMapField(field.Value, "status"); err != nil {
				r.l.Error(err, "could not set IPAllocation.status")
				meta.SetStatusCondition(prConditions, metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse,
					Reason: "ResourceSpecErr", Message: err.Error()})
				return prResources, pkgBuf, err
			}
			// update the ipam allocation
			pkgBuf.Nodes[i] = n

			// we always update the status to reflect the latest allocations
			r.l.Info("setting condition", "conditionType", conditionType)
			meta.SetStatusCondition(prConditions, metav1.Condition{Type: conditionType, Status: metav1.ConditionTrue,
				Reason: "ResourceInjected", Message: "Injected IP allocation"})
		}
	}

	return prResources, pkgBuf, nil
}

// copied from package deployment controller - clearly we need some libraries or
// to directly use the K8s meta types
func convertConditions(conditions []porchv1alpha1.Condition) *[]metav1.Condition {
	var result []metav1.Condition
	for _, c := range conditions {
		result = append(result, metav1.Condition{
			Type:    c.Type,
			Reason:  c.Reason,
			Status:  metav1.ConditionStatus(c.Status),
			Message: c.Message,
		})
	}
	return &result
}

func conditionsToKptfile(conditions *[]metav1.Condition) []kptfile.Condition {
	var prConditions []kptfile.Condition
	for _, c := range *conditions {
		prConditions = append(prConditions, kptfile.Condition{
			Type:    c.Type,
			Reason:  c.Reason,
			Status:  kptfile.ConditionStatus(c.Status),
			Message: c.Message,
		})
	}
	return prConditions
}

func unconvertConditions(conditions *[]metav1.Condition) []porchv1alpha1.Condition {
	var prConditions []porchv1alpha1.Condition
	for _, c := range *conditions {
		prConditions = append(prConditions, porchv1alpha1.Condition{
			Type:    c.Type,
			Reason:  c.Reason,
			Status:  porchv1alpha1.ConditionStatus(c.Status),
			Message: c.Message,
		})
	}

	return prConditions
}

/*
func isConditionSatisfied(conditions []porchv1alpha1.Condition, ct string) bool {
	for _, c := range conditions {
		if c.Type == ct {
			return c.Status == porchv1alpha1.ConditionTrue
		}
	}
	return false
}
*/

func getIpAllocationSpec(rn *kyaml.RNode) (*ipamv1alpha1.IPAllocationSpec, error) {
	o, err := fn.ParseKubeObject([]byte(rn.MustString()))
	if err != nil {
		return nil, err
	}

	ipAlloc := ipam.IpamAllocation{
		Obj: *o,
	}
	return ipAlloc.GetSpec()
}

func getGrpcAllocationSpec(ipAllocSpec *ipamv1alpha1.IPAllocationSpec) (*allocpb.Spec, error) {
	allocSpec := &allocpb.Spec{
		Prefixkind: ipAllocSpec.PrefixKind,
		Selector:   ipAllocSpec.Selector.MatchLabels,
	}
	switch ipAllocSpec.PrefixKind {
	case string(ipamv1alpha1.PrefixKindAggregate):
	case string(ipamv1alpha1.PrefixKindLoopback):
	case string(ipamv1alpha1.PrefixKindNetwork):
	case string(ipamv1alpha1.PrefixKindPool):
		allocSpec.PrefixLength = uint32(ipAllocSpec.PrefixLength)
	default:
		return nil, fmt.Errorf("unknown prefixkind: %s", ipAllocSpec.PrefixKind)
	}
	return allocSpec, nil
}

func GetUpdatedAllocation(resp *allocpb.Response, prefixKind ipamv1alpha1.PrefixKind) (*kyaml.RNode, error) {
	// update prefix status with the allocated prefix
	ipAlloc := &ipamv1alpha1.IPAllocation{
		Status: ipamv1alpha1.IPAllocationStatus{
			AllocatedPrefix: resp.GetAllocatedPrefix(),
		},
	}

	switch prefixKind {
	case ipamv1alpha1.PrefixKindAggregate:
	case ipamv1alpha1.PrefixKindLoopback:
	case ipamv1alpha1.PrefixKindNetwork:
		// update gateway status with the allocated gateway
		// only relevant for prefixkind = network
		ipAlloc.Status.Gateway = resp.GetGateway()
	case ipamv1alpha1.PrefixKindPool:
	}

	b, err := kyaml.Marshal(ipAlloc)
	if err != nil {
		return nil, err
	}

	return kyaml.Parse(string(b))
}
