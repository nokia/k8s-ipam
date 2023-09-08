package vlan

import (
	"fmt"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"k8s.io/utils/ptr"
)

func applyHandlerDynamicVlan(entries db.Entries[uint16], claim *vlanv1alpha1.VLANClaim) error {
	if len(entries) > 1 {
		return fmt.Errorf("claim for single entry returned multiple: %v", entries)
	}
	// update the status
	claim.Status.VLANID = ptr.To[uint16](entries[0].ID())
	return nil
}

func applyHandlerStaticVlan(entries db.Entries[uint16], claim *vlanv1alpha1.VLANClaim) error {
	if len(entries) > 1 {
		return fmt.Errorf("claim for single entry returned multiple: %v", entries)
	}
	// update the status

	if claim.Status.VLANID == nil || *claim.Status.VLANID == entries[0].ID() {
		claim.Status.VLANID = ptr.To[uint16](entries[0].ID())
	} else {
		return fmt.Errorf("vlan claim with a different vlan ID")
	}
	return nil
}

func applyHandlerMultipleVlan(entries db.Entries[uint16], claim *vlanv1alpha1.VLANClaim) error {
	// TODO update the vlan status with the proper response
	// TODO check if they match the claim
	return nil
}

func applyHandlerNewDynamicVlan(table db.DB[uint16], vctx *vlanv1alpha1.VLANClaimCtx, claim *vlanv1alpha1.VLANClaim) error {
	e, err := table.FindFree()
	if err != nil {
		return err
	}
	e = db.NewEntry(e.ID(), claim.GetUserDefinedLabels())
	if err := table.Set(e); err != nil {
		return err
	}
	claim.Status.VLANID = ptr.To[uint16](e.ID())
	return nil
}

func applyHandlerNewStaticVlan(table db.DB[uint16], vctx *vlanv1alpha1.VLANClaimCtx, claim *vlanv1alpha1.VLANClaim) error {
	e, err := table.FindFreeID(vctx.Start)
	if err != nil {
		return err
	}
	e = db.NewEntry(e.ID(), claim.GetUserDefinedLabels())
	if err := table.Set(e); err != nil {
		return err
	}
	claim.Status.VLANID = ptr.To[uint16](e.ID())
	return nil
}

func applyHandlerNewVlanRange(table db.DB[uint16], vctx *vlanv1alpha1.VLANClaimCtx, claim *vlanv1alpha1.VLANClaim) error {
	_, err := table.FindFreeRange(vctx.Start, vctx.Size)
	if err != nil {
		return err
	}
	claim.Status.VLANRange = ptr.To[string](fmt.Sprintf("%d:%d", vctx.Start, vctx.Start+vctx.Size-1))
	return nil
}

func applyHandlerNewVlanSize(table db.DB[uint16], vctx *vlanv1alpha1.VLANClaimCtx, claim *vlanv1alpha1.VLANClaim) error {
	_, err := table.FindFreeSize(vctx.Size)
	if err != nil {
		return err
	}
	claim.Status.VLANRange = ptr.To[string]("TBD update status ")
	return nil
}
