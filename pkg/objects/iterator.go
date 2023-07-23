/*
Copyright 2023 The Nephio Authors.

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

package objects

type iterator[T any] struct {
	curIdx int
	items  []T
}

func (r *iterator[T]) HasNext() bool {
	r.curIdx++
	return r.curIdx < len(r.items)
}

func (r *iterator[T]) Value() T {
	return r.items[r.curIdx]
}
