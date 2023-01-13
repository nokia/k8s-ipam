package ipam

import (
	"github.com/hansthienpondt/nipam/pkg/table"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
)

type CallbackFn func(table.Routes, allocpb.StatusCode)

type watchContext struct {
	ownerGvkKey string
	callBackFn  CallbackFn
}
