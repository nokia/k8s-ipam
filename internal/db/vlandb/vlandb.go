package vlandb

import (
	"fmt"

	"github.com/nokia/k8s-ipam/internal/db"
)

func New[T uint16]() db.DB[T] {
	return db.NewDB(&db.DBConfig[T]{
		MaxEntries: 4096,
		InitEntries: db.Entries[T]{
			db.NewEntry(T(0), map[string]string{"type": "untagged", "status": "reserved"}),
			db.NewEntry(T(1), map[string]string{"type": "default", "status": "reserved"}),
			db.NewEntry(T(4095), map[string]string{"type": "reserved", "status": "reserved"}),
		},
		SetValidation:    setVLANValidation[T],
		DeleteValidation: deleteVLANValidation[T],
	})
}

func setVLANValidation[T uint16](id T) error {
	// TODO validate max entries
	switch id {
	case 0:
		return fmt.Errorf("VLAN %d is the untagged VLAN, cannot be added to the database", id)
	case 1:
		return fmt.Errorf("VLAN %d is the default VLAN, cannot be added to the database", id)
	case 4095:
		return fmt.Errorf("VLAN %d is reserved, cannot be added to the database", id)
	}
	return nil
}

func deleteVLANValidation[T uint16](id T) error {
	// TODO validate max entries
	switch id {
	case 0:
		return fmt.Errorf("VLAN %d is the untagged VLAN, cannot be deleted from the database", id)
	case 1:
		return fmt.Errorf("VLAN %d is the default VLAN, cannot be deleted from the database", id)
	case 4095:
		return fmt.Errorf("VLAN %d is reserved, cannot be deleted from the database", id)
	}
	return nil
}
