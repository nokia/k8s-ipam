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

	corev1 "k8s.io/api/core/v1"
)

type Storage[T1, T2 any] interface {
	Restore(ctx context.Context, ref corev1.ObjectReference) error
	// only used in configmap
	SaveAll(ctx context.Context, ref corev1.ObjectReference) error
	// Destroy removes the store db
	Destroy(ctx context.Context, ref corev1.ObjectReference) error

	Get(ctx context.Context, alloc T1) ([]T2, error)
	Set(ctx context.Context, alloc T1) error
	Delete(ctx context.Context, alloc T1) error
}

func NewNopStorage[T1, T2 any]() Storage[T1, T2] {
	return &nopStorage[T1, T2]{}
}

type nopStorage[T1, T2 any] struct{}

func (r *nopStorage[T1, T2]) Restore(ctx context.Context, ref corev1.ObjectReference) error {
	return nil
}
func (r *nopStorage[T1, T2]) SaveAll(ctx context.Context, ref corev1.ObjectReference) error {
	return nil
}
func (r *nopStorage[T1, T2]) Destroy(ctx context.Context, ref corev1.ObjectReference) error {
	return nil
}
func (r *nopStorage[T1, T2]) Get(ctx context.Context, alloc T1) ([]T2, error) { return nil, nil }
func (r *nopStorage[T1, T2]) Set(ctx context.Context, alloc T1) error         { return nil }
func (r *nopStorage[T1, T2]) Delete(ctx context.Context, alloc T1) error      { return nil }
