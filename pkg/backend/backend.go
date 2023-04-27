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
)

type Backend interface {
	// CreateIndex creates a backend index
	CreateIndex(ctx context.Context, cr []byte) error
	// DeleteIndex deletes a backend index
	DeleteIndex(ctx context.Context, cr []byte) error
	// List the data from the backend index
	List(ctx context.Context, cr []byte) (any, error)
	// Add a dynamic watch with callback to the backend index
	AddWatch(ownerGvkKey, ownerGvk string, fn CallbackFn)
	// Delete a dynamic watch with callback deom the backend index
	DeleteWatch(ownerGvkKey, ownerGvk string)
	//GetAllocation return the allocation if it exists
	GetAllocation(ctx context.Context, cr []byte) ([]byte, error)
	// Allocate allocates an entry in the backend index
	Allocate(ctx context.Context, cr []byte) ([]byte, error)
	// DeAllocate deallocates an entry in the backend index
	DeAllocate(ctx context.Context, cr []byte) error
}
