package vxlan

import (
	"fmt"

	vxlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vxlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"k8s.io/utils/ptr"
)

func applyHandlerDynamicVXLAN(entries db.Entries[uint32], claim *vxlanv1alpha1.VXLANClaim) error {
	if len(entries) > 1 {
		return fmt.Errorf("claim for single entry returned multiple: %v", entries)
	}
	// update the status
	claim.Status.VXLANID = ptr.To[uint32](entries[0].ID())
	return nil
}

func applyHandlerNewDynamicVXLAN(table db.DB[uint32], vctx *vxlanv1alpha1.VXLANClaimCtx, claim *vxlanv1alpha1.VXLANClaim) error {
	e, err := table.FindFree()
	if err != nil {
		return err
	}
	e = db.NewEntry(e.ID(), claim.GetUserDefinedLabels())
	if err := table.Set(e); err != nil {
		return err
	}
	claim.Status.VXLANID = ptr.To[uint32](e.ID())
	return nil
}
