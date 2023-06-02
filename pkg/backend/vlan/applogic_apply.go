package vlan

import (
	"fmt"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	"k8s.io/utils/pointer"
)

func applyHandlerDynamicVlan(entries db.Entries[uint16], alloc *vlanv1alpha1.VLANAllocation) error {
	if len(entries) > 1 {
		return fmt.Errorf("allocate for single entry returned multiple: %v", entries)
	}
	// update the status
	alloc.Status.VLANID = util.PointerUint16(entries[0].ID())
	return nil
}

func applyHandlerStaticVlan(entries db.Entries[uint16], alloc *vlanv1alpha1.VLANAllocation) error {
	if len(entries) > 1 {
		return fmt.Errorf("allocate for single entry returned multiple: %v", entries)
	}
	// update the status

	if alloc.Status.VLANID == nil || *alloc.Status.VLANID == entries[0].ID() {
		alloc.Status.VLANID = util.PointerUint16(entries[0].ID())
	} else {
		return fmt.Errorf("vlan allocated with a different vlan ID")
	}
	return nil
}

func applyHandlerMultipleVlan(entries db.Entries[uint16], alloc *vlanv1alpha1.VLANAllocation) error {
	// TODO update the vlan status with the proper response
	// TODO check if they match the allocation
	return nil
}

func applyHandlerNewDynamicVlan(table db.DB[uint16], vctx *vlanv1alpha1.VLANAllocationCtx, alloc *vlanv1alpha1.VLANAllocation) error {
	e, err := table.FindFree()
	if err != nil {
		return err
	}
	e = db.NewEntry(e.ID(), alloc.GetUserDefinedLabels())
	if err := table.Set(e); err != nil {
		return err
	}
	alloc.Status.VLANID = util.PointerUint16(e.ID())
	return nil
}

func applyHandlerNewStaticVlan(table db.DB[uint16], vctx *vlanv1alpha1.VLANAllocationCtx, alloc *vlanv1alpha1.VLANAllocation) error {
	e, err := table.FindFreeID(vctx.Start)
	if err != nil {
		return err
	}
	e = db.NewEntry(e.ID(), alloc.GetUserDefinedLabels())
	if err := table.Set(e); err != nil {
		return err
	}
	alloc.Status.VLANID = util.PointerUint16(e.ID())
	return nil
}

func applyHandlerNewVlanRange(table db.DB[uint16], vctx *vlanv1alpha1.VLANAllocationCtx, alloc *vlanv1alpha1.VLANAllocation) error {
	_, err := table.FindFreeRange(vctx.Start, vctx.Size)
	if err != nil {
		return err
	}
	alloc.Status.VLANRange = pointer.String(fmt.Sprintf("%d:%d", vctx.Start, vctx.Start+vctx.Size-1))
	return nil
}

func applyHandlerNewVlanSize(table db.DB[uint16], vctx *vlanv1alpha1.VLANAllocationCtx, alloc *vlanv1alpha1.VLANAllocation) error {
	_, err := table.FindFreeSize(vctx.Size)
	if err != nil {
		return err
	}
	alloc.Status.VLANRange = pointer.String("TBD update status ")
	return nil
}
