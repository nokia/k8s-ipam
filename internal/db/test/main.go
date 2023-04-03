package main

import (
	"fmt"

	"github.com/nokia/k8s-ipam/internal/db"
	"github.com/nokia/k8s-ipam/internal/db/vlandb"
)

func main() {
	d := vlandb.New()
	d.Set(db.NewEntry(uint16(9), map[string]string{"cluster": "cluster1"}))
	d.Set(db.NewEntry(uint16(10), map[string]string{"cluster": "cluster1"}))
	d.Set(db.NewEntry(uint16(11), map[string]string{"cluster": "cluster1"}))

	entries := d.GetAll()

	for _, e := range entries {
		fmt.Println("entry:", e.ID(), e.Labels())
	}
}
