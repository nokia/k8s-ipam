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

package vlanspecializer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	kptv1 "github.com/GoogleContainerTools/kpt/pkg/api/kptfile/v1"
	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/go-logr/logr"
	kptfilelibv1 "github.com/nephio-project/nephio/krm-functions/lib/kptfile/v1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/resource"
	"github.com/nokia/k8s-ipam/internal/shared"
	"github.com/nokia/k8s-ipam/pkg/fn/vlan/function"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

// +kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/status,verbs=get;update;patch
// SetupWithManager sets up the controller with the Manager.
func Setup(mgr ctrl.Manager, options *shared.Options) error {
	//ge := make(chan event.GenericEvent)
	r := &reconciler{
		Client:          mgr.GetClient(),
		porchClient:     options.PorchClient,
		VlanClientProxy: options.VlanClientProxy,
	}

	// TBD how does the proxy cache work with the injector for updates
	return ctrl.NewControllerManagedBy(mgr).
		For(&porchv1alpha1.PackageRevision{}).
		//Watches(&source.Channel{Source: ge}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// reconciler reconciles a NetworkInstance object
type reconciler struct {
	client.Client
	porchClient     client.Client
	VlanClientProxy clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation]

	l logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx).WithValues("req", req)
	r.l.Info("reconcile specializer")

	pr := &porchv1alpha1.PackageRevision{}
	if err := r.Get(ctx, req.NamespacedName, pr); err != nil {
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
		APIVersion: vlanv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
		Kind:       vlanv1alpha1.VLANAllocationKind,
	})
	if hasSpecificTypeConditions(pr.Status.Conditions, ct) {
		// get package revision resourceList
		prr := &porchv1alpha1.PackageRevisionResources{}
		if err := r.porchClient.Get(ctx, req.NamespacedName, prr); err != nil {
			r.l.Error(err, "cannot get package revision resources")
			return ctrl.Result{}, errors.Wrap(err, "cannot get package revision resources")
		}
		// create resourceList
		rl, err := r.getResourceList(prr.Spec.Resources)
		if err != nil {
			r.l.Error(err, "cannot get resourceList")
			return ctrl.Result{}, errors.Wrap(err, "cannot get resourceList")
		}

		fnr := function.FnR{VlanClientProxy: r.VlanClientProxy}
		_, err = fnr.Run(rl)
		if err != nil {
			r.l.Error(err, "function run failed")
			// TBD if we need to return here + check if kptfile is set
			//return ctrl.Result{}, errors.Wrap(err, "function run failed")
		}
		for _, o := range rl.Items {
			r.l.Info("resourceList", "data", o.String())
			// TBD what if we create new resources
			// update the resources with the latest info
			prr.Spec.Resources[o.GetAnnotation(kioutil.PathAnnotation)] = o.String()
		}
		kptfile := rl.Items.GetRootKptfile()
		if kptfile == nil {
			r.l.Error(fmt.Errorf("mandatory Kptfile is missing from the package"), "")
			return ctrl.Result{}, nil
		}

		kptf, err := kptfilelibv1.New(rl.Items.GetRootKptfile().String())
		if err != nil {
			r.l.Error(err, "cannot unmarshal kptfile")
			return ctrl.Result{}, nil
		}
		pr.Status.Conditions = getPorchCondiitons(kptf.GetConditions())
		if err = r.porchClient.Update(ctx, prr); err != nil {
			return ctrl.Result{}, err
		}

	}
	return ctrl.Result{}, nil
}

func getPorchCondiitons(cs []kptv1.Condition) []porchv1alpha1.Condition {
	var prConditions []porchv1alpha1.Condition
	for _, c := range cs {
		prConditions = append(prConditions, porchv1alpha1.Condition{
			Type:    c.Type,
			Reason:  c.Reason,
			Status:  porchv1alpha1.ConditionStatus(c.Status),
			Message: c.Message,
		})
	}
	return prConditions
}

// hasSpecificTypeConditions checks if the kptfile has ipam conditions
// we dont care if the conditions are true or false as we refresh the ipam allocations
// in this way
func hasSpecificTypeConditions(conditions []porchv1alpha1.Condition, conditionType string) bool {
	for _, c := range conditions {
		if strings.HasPrefix(c.Type, conditionType+".") {
			return true
		}
	}
	return false
}

func (r *reconciler) includeFile(path string, match []string) bool {
	for _, m := range match {
		file := filepath.Base(path)
		if matched, err := filepath.Match(m, file); err == nil && matched {
			return true
		}
	}
	return false
}

func (r *reconciler) getResourceList(resources map[string]string) (*fn.ResourceList, error) {
	inputs := []kio.Reader{}
	for path, data := range resources {
		if r.includeFile(path, []string{"*.yaml", "*.yml", "Kptfile"}) {
			inputs = append(inputs, &kio.ByteReader{
				Reader: strings.NewReader(data),
				SetAnnotations: map[string]string{
					kioutil.PathAnnotation: path,
				},
				DisableUnwrapping: true,
			})
		}
	}
	var pb kio.PackageBuffer
	err := kio.Pipeline{
		Inputs:  inputs,
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{&pb},
	}.Execute()
	if err != nil {
		return nil, err
	}

	rl := &fn.ResourceList{
		Items: fn.KubeObjects{},
	}
	for _, n := range pb.Nodes {
		s, err := n.String()
		if err != nil {
			return nil, err
		}
		o, err := fn.ParseKubeObject([]byte(s))
		if err != nil {
			r.l.Error(err, "cannot parse object", "apiversion", n.GetApiVersion(), "kind", n.GetKind())
			return nil, err
		}
		if err := rl.UpsertObjectToItems(o, nil, true); err != nil {
			r.l.Error(err, "cannot insert object in rl", "apiversion", n.GetApiVersion(), "kind", n.GetKind())
			return nil, err
		}
	}
	return rl, nil
}
