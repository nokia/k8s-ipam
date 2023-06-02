/*
Copyright 2022 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vlan

import (
	"context"

	"github.com/hansthienpondt/nipam/pkg/table"
	"github.com/nokia/k8s-ipam/pkg/alloc/allocpb"
	"github.com/nokia/k8s-ipam/pkg/backend"
)

type CallbackFn func(table.Routes, allocpb.StatusCode)

type Watcher interface {
	addWatch(ownerGvkKey, ownerGvk string, fn backend.CallbackFn)
	deleteWatch(ownerGvkKey, ownerGvk string)
	handleUpdate(ctx context.Context, routes table.Routes, statusCode allocpb.StatusCode)
}
