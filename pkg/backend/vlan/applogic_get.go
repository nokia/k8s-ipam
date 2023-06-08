package vlan

import (
	"fmt"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
)

func getHandlerSingleVlan(entries db.Entries[uint16], claim *vlanv1alpha1.VLANClaim) error {
	if len(entries) > 1 {
		return fmt.Errorf("get for single entry returned multiple: %v", entries)
	}
	// update the status
	claim.Status.VLANID = util.PointerUint16(entries[0].ID())
	return nil
}

func getHandlerMultipleVlan(entries db.Entries[uint16], claim *vlanv1alpha1.VLANClaim) error {
	// TODO update the vlan status with the proper response
	return nil
}
