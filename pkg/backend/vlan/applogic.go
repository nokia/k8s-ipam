package vlan

import (
	"context"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/db"
	"github.com/nokia/k8s-ipam/pkg/backend"
)

func (r *be) newApplogic(cr *vlanv1alpha1.VLANClaim, initializing bool) (backend.AppLogic[*vlanv1alpha1.VLANClaim], error) {
	// we assume right now 1 database ID
	t, err := r.cache.Get(cr.GetCacheID(), initializing)
	if err != nil {
		return nil, err
	}

	vlanClaimCtx, err := cr.GetVLANClaimCtx()
	if err != nil {
		return nil, err
	}
	r.l.Info("newApplogic", "vlanClaimCtx", vlanClaimCtx)

	return newVLANApplogic(t, vlanClaimCtx, false)

}

func newVLANApplogic(t db.DB[uint16], vctx *vlanv1alpha1.VLANClaimCtx, init bool) (backend.AppLogic[*vlanv1alpha1.VLANClaim], error) {
	r := &applogic{
		table: t,
		vctx:  vctx,
		fnc: map[vlanv1alpha1.VLANClaimType]*applogicFunctionConfig{
			vlanv1alpha1.VLANClaimTypeDynamic: {
				getHandler:        getHandlerSingleVlan,
				applyHandlerFound: applyHandlerDynamicVlan,
				applyHandlerNew:   applyHandlerNewDynamicVlan,
			},
			vlanv1alpha1.VLANClaimTypeStatic: {
				getHandler:        getHandlerSingleVlan,
				applyHandlerFound: applyHandlerStaticVlan,
				applyHandlerNew:   applyHandlerNewStaticVlan,
			},
			vlanv1alpha1.VLANClaimTypeRange: {
				getHandler:        getHandlerMultipleVlan,
				applyHandlerFound: applyHandlerMultipleVlan,
				applyHandlerNew:   applyHandlerNewVlanRange,
			},
			vlanv1alpha1.VLANClaimTypeSize: {
				getHandler:        getHandlerMultipleVlan,
				applyHandlerFound: applyHandlerMultipleVlan,
				applyHandlerNew:   applyHandlerNewVlanSize,
			},
		},
	}

	return backend.NewApplogic(&backend.ApplogicConfig[*vlanv1alpha1.VLANClaim]{
		GetHandler:      r.GetHandler,
		ValidateHandler: r.ValidateHandler,
		ApplyHandler:    r.ApplyHandler,
		DeleteHandler:   r.DeleteHandler,
	})
}

type applogic struct {
	table db.DB[uint16]
	vctx  *vlanv1alpha1.VLANClaimCtx
	fnc   map[vlanv1alpha1.VLANClaimType]*applogicFunctionConfig
}

type applogicFunctionConfig struct {
	getHandler        func(entries db.Entries[uint16], claim *vlanv1alpha1.VLANClaim) error
	applyHandlerFound func(entries db.Entries[uint16], claim *vlanv1alpha1.VLANClaim) error
	applyHandlerNew   func(table db.DB[uint16], vctx *vlanv1alpha1.VLANClaimCtx, claim *vlanv1alpha1.VLANClaim) error
}

func (r *applogic) GetHandler(ctx context.Context, a *vlanv1alpha1.VLANClaim) (*vlanv1alpha1.VLANClaim, error) {
	// get the entries in the table based on the lebels in the spec
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

func (r *applogic) ValidateHandler(ctx context.Context, a *vlanv1alpha1.VLANClaim) (string, error) {
	// no validatopn required so far
	return "", nil
}

func (r *applogic) ApplyHandler(ctx context.Context, a *vlanv1alpha1.VLANClaim) (*vlanv1alpha1.VLANClaim, error) {
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

func (r *applogic) DeleteHandler(ctx context.Context, a *vlanv1alpha1.VLANClaim) error {
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

func (r *applogic) getEntriesByOwner(t db.DB[uint16], a *vlanv1alpha1.VLANClaim) (db.Entries[uint16], error) { // get vlan by owner
	ownerSelector, err := a.GetOwnerSelector()
	if err != nil {
		return nil, err
	}
	entries := t.GetByLabel(ownerSelector)
	if len(entries) != 0 {
		return entries, nil
	}
	return db.Entries[uint16]{}, nil
}
