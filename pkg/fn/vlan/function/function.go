package function

import (
	"context"
	"fmt"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/nephio-project/nephio/krm-functions/lib/condkptsdk"
	"github.com/nephio-project/nephio/krm-functions/lib/kubeobject"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	corev1 "k8s.io/api/core/v1"
)

type FnR struct {
	VlanClientProxy clientproxy.Proxy[*vlanv1alpha1.VLANDatabase, *vlanv1alpha1.VLANAllocation]
}

func (r *FnR) Run(rl *fn.ResourceList) (bool, error) {
	sdk, err := condkptsdk.New(
		rl,
		&condkptsdk.Config{
			For: corev1.ObjectReference{
				APIVersion: vlanv1alpha1.GroupVersion.Identifier(),
				Kind:       vlanv1alpha1.VLANAllocationKind,
			},
			PopulateOwnResourcesFn: nil,
			GenerateResourceFn:     r.updateVLANAllocationResource,
		},
	)
	if err != nil {
		rl.Results.ErrorE(err)
		return false, nil
	}
	return sdk.Run()
}

func (r *FnR) updateVLANAllocationResource(forObj *fn.KubeObject, objs fn.KubeObjects) (*fn.KubeObject, error) {
	if forObj == nil {
		return nil, fmt.Errorf("expected a for object but got nil")
	}
	fn.Logf("vlanalloc: %v\n", forObj)
	allocKOE, err := kubeobject.NewFromKubeObject[*vlanv1alpha1.VLANAllocation](forObj)
	if err != nil {
		return nil, err
	}
	alloc, err := allocKOE.GetGoStruct()
	if err != nil {
		return nil, err
	}
	resp, err := r.VlanClientProxy.Allocate(context.Background(), alloc, nil)
	if err != nil {
		return nil, err
	}
	alloc.Status = resp.Status

	if alloc.Status.VLANID != nil {
		fn.Logf("alloc resp vlan: %v\n", *resp.Status.VLANID)
	}
	// set the status
	err = allocKOE.SetStatus(resp.Status)
	return &allocKOE.KubeObject, err
}
