package vxlandb

import (
	"fmt"

	"github.com/nokia/k8s-ipam/pkg/db"
)

type Config[T uint32] struct {
	Offset     T
	MaxEntryID T
}

func New[T uint32](cfg *Config[T]) db.DB[T] {
	r := &vxlan[T]{cfg: cfg}
	return db.NewDB(&db.DBConfig[T]{
		Offset:           cfg.Offset,
		MaxEntries:       cfg.MaxEntryID-cfg.Offset,
		SetValidation:    r.setVLANValidation,
		DeleteValidation: r.deleteVLANValidation,
	})
}

type vxlan[T uint32] struct {
	cfg *Config[T]
}

func (r *vxlan[T]) setVLANValidation(id T) error {
	if id < r.cfg.Offset {
		return fmt.Errorf("VXLAN %d is lower than the lowest offset %d", id, r.cfg.Offset)
	}
	if id > r.cfg.MaxEntryID {
		return fmt.Errorf("VXLAN %d is higher than the max offset %d", id, r.cfg.MaxEntryID)
	}
	return nil
}

func (r *vxlan[T]) deleteVLANValidation(id T) error {
	if id < r.cfg.Offset {
		return fmt.Errorf("VXLAN %d is lower than the lowest offset %d", id, r.cfg.Offset)
	}
	if id > r.cfg.MaxEntryID {
		return fmt.Errorf("VXLAN %d is higher than the max offset %d", id, r.cfg.MaxEntryID)
	}
	return nil
}
