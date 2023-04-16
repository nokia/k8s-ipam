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

package backend

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Backend[T1 client.Object, T2, T3 any] interface {
	// Create and initialize a backend Instance db
	Create(ctx context.Context, cr T1) error
	// Delete the backend instance db
	Delete(ctx context.Context, cr T1) error
	// List the data from the backend instance db
	List(ctx context.Context, cr T1) (T3, error)
	// Add a dynamic watch with callback to the backend db
	AddWatch(ownerGvkKey, ownerGvk string, fn CallbackFn)
	// Delete a dynamic watch with callback to the backend db
	DeleteWatch(ownerGvkKey, ownerGvk string)
	//GetAllocatedPrefix return the requested allocated prefix
	Get(ctx context.Context, cr T2) (T2, error)
	// Allocate allocates an intry in the backend db
	Allocate(ctx context.Context, cr T2) (T2, error)
	// DeAllocateIPPrefix deallocates the allocation based on owner selection. No errors are returned if no allocation was found
	DeAllocate(ctx context.Context, cr T2) error
}
