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

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlrconfig"
	"github.com/nokia/k8s-ipam/controllers/specializerreconciler"
	function "github.com/nokia/k8s-ipam/pkg/fn/vlan-fn/fn"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/vlan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func init() {
	controllers.Register("vlanspecializer", &reconciler{})
}

// +kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/status,verbs=get;update;patch
// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) Setup(ctx context.Context, mgr ctrl.Manager, cfg *ctrlrconfig.ControllerConfig) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {
	// register scheme
	if err := porchv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return nil, err
	}

	fnr := &function.FnR{ClientProxy: vlan.New(
		ctx, clientproxy.Config{Address: cfg.Address},
	)}

	r.Client = mgr.GetClient()
	r.PorchClient = cfg.PorchClient
	r.For = corev1.ObjectReference{
		APIVersion: vlanv1alpha1.SchemeBuilder.GroupVersion.Identifier(),
		Kind:       vlanv1alpha1.VLANAllocationKind,
	}
	r.Krmfn = fn.ResourceListProcessorFunc(fnr.Run)

	return nil, ctrl.NewControllerManagedBy(mgr).
		For(&porchv1alpha1.PackageRevision{}).
		Complete(r)

}

// reconciler reconciles a object
type reconciler struct {
	specializerreconciler.Reconciler
}
