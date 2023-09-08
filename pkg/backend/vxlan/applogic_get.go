package vxlan

import (
	"fmt"

	vxlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vxlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"k8s.io/utils/ptr"
)

func getHandlerSingleVXLAN(entries db.Entries[uint32], claim *vxlanv1alpha1.VXLANClaim) error {
	if len(entries) > 1 {
		return fmt.Errorf("get for single entry returned multiple: %v", entries)
	}
	// update the status
	claim.Status.VXLANID = ptr.To[uint32](entries[0].ID())
	return nil
}

func getHandlerMultipleVXLAN(entries db.Entries[uint16], claim *vxlanv1alpha1.VXLANClaim) error {
	// TODO update the vlan status with the proper response
	return nil
}
