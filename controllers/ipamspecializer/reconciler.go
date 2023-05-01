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

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	kptfile "github.com/GoogleContainerTools/kpt/pkg/api/kptfile/v1"
	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/nephio-project/nephio-controller-poc/pkg/porch"
	kptfilelibv1 "github.com/nephio-project/nephio/krm-functions/lib/kptfile/v1"
	"github.com/nokia/k8s-ipam/apis/alloc/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/resource"
	"github.com/nokia/k8s-ipam/internal/shared"
	"github.com/nokia/k8s-ipam/internal/utils/util"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	ipamConditionType = "ipam.nephio.org.IPAMAllocation"
)

//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func Setup(mgr ctrl.Manager, options *shared.Options) error {
	ge := make(chan event.GenericEvent)
	r := &reconciler{
		kind:            "ipam",
		Client:          mgr.GetClient(),
		porchClient:     options.PorchClient,
		IpamClientProxy: options.IpamClientProxy,
	}

	// TBD how does the proxy cache work with the injector for updates
	return ctrl.NewControllerManagedBy(mgr).
		For(&porchv1alpha1.PackageRevision{}).
		Watches(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// reconciler reconciles a NetworkInstance object
type reconciler struct {
	kind string
	client.Client
	porchClient     client.Client
	IpamClientProxy clientproxy.Proxy[*ipamv1alpha1.NetworkInstance, *ipamv1alpha1.IPAllocation]

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
	ct := kptfilelibv1.GetConditionType(&corev1.ObjectReference{
		APIVersion: ipamv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
		Kind:       ipamv1alpha1.IPAllocationKind,
	})
	ipamConditions := hasIPAMConditions(cr.Status.Conditions, ct)

	if len(ipamConditions) > 0 {
		r.l.Info("run injector", "pr", cr.GetName())
		if err := r.allocateIPs(ctx, req.NamespacedName); err != nil {
			r.l.Error(err, "injection error")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// hasIPAMConditions checks if the kptfile has ipam conditions
// we dont care if the conditions are true or false as we refresh the ipam allocations
// in this way
func hasIPAMConditions(conditions []porchv1alpha1.Condition, conditionType string) []porchv1alpha1.Condition {
	var uc []porchv1alpha1.Condition
	for _, c := range conditions {
		if strings.HasPrefix(c.Type, conditionType+".") {
			uc = append(uc, c)
		}
	}
	return uc
}

func (r *reconciler) allocateIPs(ctx context.Context, namespacedName types.NamespacedName) error {
	r.l = log.FromContext(ctx)
	r.l.Info("specializer", "name", namespacedName.String())

	origPr := &porchv1alpha1.PackageRevision{}
	if err := r.porchClient.Get(ctx, namespacedName, origPr); err != nil {
		return err
	}

	pr := origPr.DeepCopy()

	//r.l.Info("injector function", "name", namespacedName.String(), "pr spec", pr.Spec)

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

func (r *reconciler) injectAllocatedIPs(
	ctx context.Context,
	namespacedName types.NamespacedName,
	prConditions *[]metav1.Condition,
	pr *porchv1alpha1.PackageRevision,
) (*porchv1alpha1.PackageRevisionResources, *kio.PackageBuffer, error) {

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
			ipAlloc, err := getIpAllocation(rn)
			if err != nil {
				return prResources, pkgBuf, fmt.Errorf("cannot convert ip allocation to a typed spec: %s", rn.GetName())
			}

			resp, err := r.IpamClientProxy.Allocate(ctx, ipAlloc, nil)
			if err != nil {
				r.l.Error(err, "grpc ipam allocation request error", "ipAlloc", ipAlloc)
				return prResources, pkgBuf, errors.Wrap(err, "cannot allocate ip")
			}

			r.l.Info("grpc ipam allocation response", "Name", rn.GetName(), "resp", resp)

			// update Allocation
			ipAllocation, err := GetUpdatedAllocation(resp, ipamv1alpha1.PrefixKind(ipAlloc.Spec.Kind))
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

type IpamAllocation struct {
	Obj fn.KubeObject
}

func (r *IpamAllocation) GetIPAllocation() (*ipamv1alpha1.IPAllocation, error) {
	spec, err := r.GetSpec()
	if err != nil {
		return nil, err
	}

	return &ipamv1alpha1.IPAllocation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ipamv1alpha1.GroupVersion.String(),
			Kind:       ipamv1alpha1.IPAllocationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.Obj.GetNamespace(),
			Name:      r.Obj.GetName(),
			Labels:    r.Obj.GetLabels(),
		},
		Spec: *spec,
	}, nil
}

func (r *IpamAllocation) GetSpec() (*ipamv1alpha1.IPAllocationSpec, error) {
	spec := r.Obj.GetMap("spec")
	selectorLabels, _, err := spec.NestedStringMap("selector", "matchLabels")
	if err != nil {
		return nil, err
	}
	networkInstanceRef := spec.GetMap("networkInstanceRef")

	creatPrefix := false
	prefixLength := int(spec.GetInt("prefixLength"))
	if prefixLength > 0 {
		creatPrefix = true
	}

	ipAllocSpec := &ipamv1alpha1.IPAllocationSpec{
		Kind: ipamv1alpha1.PrefixKind(spec.GetString("kind")),
		NetworkInstance: corev1.ObjectReference{
			Namespace: networkInstanceRef.GetString("namespace"),
			Name:      networkInstanceRef.GetString("name"),
		},
		Prefix:       pointer.String(spec.GetString("prefix")),
		PrefixLength: util.PointerUint8(prefixLength),
		CreatePrefix: pointer.Bool(creatPrefix),
		AllocationLabels: v1alpha1.AllocationLabels{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}

	return ipAllocSpec, nil
}

func getIpAllocation(rn *kyaml.RNode) (*ipamv1alpha1.IPAllocation, error) {
	o, err := fn.ParseKubeObject([]byte(rn.MustString()))
	if err != nil {
		return nil, err
	}

	ipAlloc := IpamAllocation{
		Obj: *o,
	}
	return ipAlloc.GetIPAllocation()
}

func GetUpdatedAllocation(resp *ipamv1alpha1.IPAllocation, prefixKind ipamv1alpha1.PrefixKind) (*kyaml.RNode, error) {
	// update prefix status with the allocated prefix
	ipAlloc := &ipamv1alpha1.IPAllocation{
		Status: ipamv1alpha1.IPAllocationStatus{
			Prefix: resp.Status.Prefix,
		},
	}

	switch prefixKind {
	case ipamv1alpha1.PrefixKindAggregate:
	//case ipamv1alpha1.PrefixKindLoopback:
	case ipamv1alpha1.PrefixKindNetwork:
		// update gateway status with the allocated gateway
		// only relevant for prefixkind = network
		ipAlloc.Status.Gateway = resp.Status.Gateway

	case ipamv1alpha1.PrefixKindPool:
	}

	b, err := kyaml.Marshal(ipAlloc)
	if err != nil {
		return nil, err
	}

	return kyaml.Parse(string(b))
}
