package ipam

import (
	"fmt"

	"github.com/hansthienpondt/goipam/pkg/table"
)

func newIpamInfo() *ipamInfo {
	return &ipamInfo{
		init: true,
		rt:   table.NewRouteTable(),
	}
}

func (r *ipamInfo) InitDone() {
	r.init = false
}

func (r *ipamInfo) IsInit() bool {
	return r.init
}

func (r *ipam) init(crName string) bool {
	r.m.Lock()
	defer r.m.Unlock()
	_, ok := r.ipam[crName]
	if !ok {
		r.ipam[crName] = newIpamInfo()
		return true
	}
	return false
}

func (r *ipam) initDone(crName string) error {
	r.m.Lock()
	defer r.m.Unlock()
	ii, ok := r.ipam[crName]
	if !ok {
		return fmt.Errorf("network instance not initialized: %s", crName)
	}
	ii.InitDone()
	return nil
}

func (r *ipam) get(crName string, init bool) (*table.RouteTable, error) {
	r.m.Lock()
	defer r.m.Unlock()
	ii, ok := r.ipam[crName]
	if !ok {
		return nil, fmt.Errorf("network instance not initialized: %s", crName)
	}
	if !init && ii.IsInit() {
		return nil, fmt.Errorf("network instance is initializing: %s", crName)
	}
	return ii.rt, nil
}

func (r *ipam) getRoutingTable(alloc *Allocation, dryrun, init bool) (*table.RouteTable, error) {
	rt, err := r.get(alloc.GetNetworkInstance(), init)
	if err != nil {
		return nil, err
	}
	if dryrun {
		// copy the routing table for validation
		return copyRoutingTable(rt), nil
	}
	//r.l.Info("validateIPAllocation", "ipam size", rt.Size())
	return rt, nil
}

func copyRoutingTable(rt *table.RouteTable) *table.RouteTable {
	newrt := table.NewRouteTable()
	for _, route := range rt.GetTable() {
		newrt.Add(route)
	}
	return newrt
}
