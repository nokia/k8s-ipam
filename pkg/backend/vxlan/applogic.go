package vxlan

import (
	"context"

	vxlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vxlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/pkg/backend"
)

func (r *be) newApplogic(cr *vxlanv1alpha1.VXLANClaim, initializing bool) (backend.AppLogic[*vxlanv1alpha1.VXLANClaim], error) {
	// we assume right now 1 database ID
	t, err := r.cache.Get(cr.GetCacheID(), initializing)
	if err != nil {
		return nil, err
	}

	vxlanClaimCtx, err := cr.GetVXLANClaimCtx()
	if err != nil {
		return nil, err
	}
	r.l.Info("newApplogic", "vlanClaimCtx", vxlanClaimCtx)

	return newVXLANApplogic(t, vxlanClaimCtx, false)

}

func newVXLANApplogic(t db.DB[uint32], vctx *vxlanv1alpha1.VXLANClaimCtx, init bool) (backend.AppLogic[*vxlanv1alpha1.VXLANClaim], error) {
	r := &applogic{
		table: t,
		vctx:  vctx,
		fnc: map[vxlanv1alpha1.VXLANClaimType]*applogicFunctionConfig{
			vxlanv1alpha1.VXLANClaimTypeDynamic: {
				getHandler:        getHandlerSingleVXLAN,
				applyHandlerFound: applyHandlerDynamicVXLAN,
				applyHandlerNew:   applyHandlerNewDynamicVXLAN,
			},
		},
	}

	return backend.NewApplogic(&backend.ApplogicConfig[*vxlanv1alpha1.VXLANClaim]{
		GetHandler:      r.GetHandler,
		ValidateHandler: r.ValidateHandler,
		ApplyHandler:    r.ApplyHandler,
		DeleteHandler:   r.DeleteHandler,
	})
}

type applogic struct {
	table db.DB[uint32]
	vctx  *vxlanv1alpha1.VXLANClaimCtx
	fnc   map[vxlanv1alpha1.VXLANClaimType]*applogicFunctionConfig
}

type applogicFunctionConfig struct {
	getHandler        func(entries db.Entries[uint32], claim *vxlanv1alpha1.VXLANClaim) error
	applyHandlerFound func(entries db.Entries[uint32], claim *vxlanv1alpha1.VXLANClaim) error
	applyHandlerNew   func(table db.DB[uint32], vctx *vxlanv1alpha1.VXLANClaimCtx, claim *vxlanv1alpha1.VXLANClaim) error
}

func (r *applogic) GetHandler(ctx context.Context, a *vxlanv1alpha1.VXLANClaim) (*vxlanv1alpha1.VXLANClaim, error) {
	// get the entries in the table based on the labels in the spec
	claim := a.DeepCopy()
	entries, err := r.getEntriesByOwner(r.table, a)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		if r.fnc[r.vctx.Kind].getHandler != nil {
			if err = r.fnc[r.vctx.Kind].getHandler(entries, claim); err != nil {
				return nil, err
			}
		}
	}
	return claim, nil
}

func (r *applogic) ValidateHandler(ctx context.Context, a *vxlanv1alpha1.VXLANClaim) (string, error) {
	// no validatopn required so far
	return "", nil
}

func (r *applogic) ApplyHandler(ctx context.Context, a *vxlanv1alpha1.VXLANClaim) (*vxlanv1alpha1.VXLANClaim, error) {
	claim := a.DeepCopy()
	// get the entries in the table based on the owner references
	entries, err := r.getEntriesByOwner(r.table, a)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		// entry exists
		if r.fnc[r.vctx.Kind].applyHandlerFound != nil {
			if err := r.fnc[r.vctx.Kind].applyHandlerFound(entries, claim); err != nil {
				return nil, err
			}
			return claim, nil
		}
	}
	// new claim required
	if r.fnc[r.vctx.Kind].applyHandlerNew != nil {
		if err := r.fnc[r.vctx.Kind].applyHandlerNew(r.table, r.vctx, claim); err != nil {
			return nil, err
		}
	}
	return claim, nil
}

func (r *applogic) DeleteHandler(ctx context.Context, a *vxlanv1alpha1.VXLANClaim) error {
	// get the entries in the cache based on the owner references
	entries, err := r.getEntriesByOwner(r.table, a)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := r.table.Delete(e.ID()); err != nil {
			return err
		}
	}
	return nil
}

func (r *applogic) getEntriesByOwner(t db.DB[uint32], a *vxlanv1alpha1.VXLANClaim) (db.Entries[uint32], error) { // get vxlan by owner
	ownerSelector, err := a.GetOwnerSelector()
	if err != nil {
		return nil, err
	}
	entries := t.GetByLabel(ownerSelector)
	if len(entries) != 0 {
		return entries, nil
	}
	return db.Entries[uint32]{}, nil
}
