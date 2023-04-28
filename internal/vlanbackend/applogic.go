package vlanbackend

import (
	"context"

	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/pkg/backend"
)

func (r *be) newApplogic(cr *vlanv1alpha1.VLANAllocation, initializing bool) (backend.AppLogic[*vlanv1alpha1.VLANAllocation], error) {
	// we assume right now 1 database ID
	t, err := r.cache.Get(cr.GetCacheID(), initializing)
	if err != nil {
		return nil, err
	}

	vlanAllocCtx, err := cr.GetVLANAllocationCtx()
	if err != nil {
		return nil, err
	}
	r.l.Info("newApplogic", "vlanAllocCtx", vlanAllocCtx)

	return newVLANApplogic(t, vlanAllocCtx, false)

}

func newVLANApplogic(t db.DB[uint16], vctx *vlanv1alpha1.VLANAllocationCtx, init bool) (backend.AppLogic[*vlanv1alpha1.VLANAllocation], error) {
	r := &applogic{
		table: t,
		vctx:  vctx,
		fnc: map[vlanv1alpha1.VLANAllocKind]*applogicFunctionConfig{
			vlanv1alpha1.VLANAllocKindDynamic: {
				getHandler:        getHandlerSingleVlan,
				applyHandlerFound: applyHandlerDynamicVlan,
				applyHandlerNew:   applyHandlerNewDynamicVlan,
			},
			vlanv1alpha1.VLANAllocKindStatic: {
				getHandler:        getHandlerSingleVlan,
				applyHandlerFound: applyHandlerStaticVlan,
				applyHandlerNew:   applyHandlerNewStaticVlan,
			},
			vlanv1alpha1.VLANAllocKindRange: {
				getHandler:        getHandlerMultipleVlan,
				applyHandlerFound: applyHandlerMultipleVlan,
				applyHandlerNew:   applyHandlerNewVlanRange,
			},
			vlanv1alpha1.VLANAllocKindSize: {
				getHandler:        getHandlerMultipleVlan,
				applyHandlerFound: applyHandlerMultipleVlan,
				applyHandlerNew:   applyHandlerNewVlanSize,
			},
		},
	}

	return backend.NewApplogic(&backend.ApplogicConfig[*vlanv1alpha1.VLANAllocation]{
		GetHandler:      r.GetHandler,
		ValidateHandler: r.ValidateHandler,
		ApplyHandler:    r.ApplyHandler,
		DeleteHandler:   r.DeleteHandler,
	})
}

type applogic struct {
	table db.DB[uint16]
	vctx  *vlanv1alpha1.VLANAllocationCtx
	fnc   map[vlanv1alpha1.VLANAllocKind]*applogicFunctionConfig
}

type applogicFunctionConfig struct {
	getHandler        func(entries db.Entries[uint16], alloc *vlanv1alpha1.VLANAllocation) error
	applyHandlerFound func(entries db.Entries[uint16], alloc *vlanv1alpha1.VLANAllocation) error
	applyHandlerNew   func(table db.DB[uint16], vctx *vlanv1alpha1.VLANAllocationCtx, alloc *vlanv1alpha1.VLANAllocation) error
}

func (r *applogic) GetHandler(ctx context.Context, a *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error) {
	// get the entries in the table based on the lebels in the spec
	alloc := a.DeepCopy()
	entries, err := r.getEntriesByOwner(r.table, a)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		if r.fnc[r.vctx.Kind].getHandler != nil {
			if err = r.fnc[r.vctx.Kind].getHandler(entries, alloc); err != nil {
				return nil, err
			}
		}
	}
	return alloc, nil
}

func (r *applogic) ValidateHandler(ctx context.Context, a *vlanv1alpha1.VLANAllocation) (string, error) {
	// no validatopn required so far
	return "", nil
}

func (r *applogic) ApplyHandler(ctx context.Context, a *vlanv1alpha1.VLANAllocation) (*vlanv1alpha1.VLANAllocation, error) {
	alloc := a.DeepCopy()
	// get the entries in the table based on the owner references
	entries, err := r.getEntriesByOwner(r.table, a)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		// entry exists
		if r.fnc[r.vctx.Kind].applyHandlerFound != nil {
			if err := r.fnc[r.vctx.Kind].applyHandlerFound(entries, alloc); err != nil {
				return nil, err
			}
			return alloc, nil
		}
	}
	// new allocation required
	if r.fnc[r.vctx.Kind].applyHandlerNew != nil {
		if err := r.fnc[r.vctx.Kind].applyHandlerNew(r.table, r.vctx, alloc); err != nil {
			return nil, err
		}
	}
	return alloc, nil
}

func (r *applogic) DeleteHandler(ctx context.Context, a *vlanv1alpha1.VLANAllocation) error {
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

func (r *applogic) getEntriesByOwner(t db.DB[uint16], a *vlanv1alpha1.VLANAllocation) (db.Entries[uint16], error) { // get vlan by owner
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
