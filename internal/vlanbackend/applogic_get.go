package vlanbackend

import (
	"fmt"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
)

func getHandlerSingleVlan(entries db.Entries[uint16], alloc *vlanv1alpha1.VLANAllocation) error {
	if len(entries) > 1 {
		return fmt.Errorf("get for single entry returned multiple: %v", entries)
	}
	// update the status
	alloc.Status.AllocatedVlanID = entries[0].ID()
	return nil
}

func getHandlerMultipleVlan(entries db.Entries[uint16], alloc *vlanv1alpha1.VLANAllocation) error {
	// TODO update the vlan status with the proper response
	return nil
}
