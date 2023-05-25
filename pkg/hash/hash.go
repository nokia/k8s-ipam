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

package hash

import (
	"k8s.io/apimachinery/pkg/labels"
)

type HashTable interface {
	Insert(string, string, map[string]string) uint32
	Delete(string, string, map[string]string)
	GetAllocated() (uint32, []*string)
}

type node struct {
	key      string
	register map[string]labels.Set
}

type hashTable struct {
	size  uint32
	nodes []*node
}

func New(s uint32) HashTable {
	h := &hashTable{
		size:  s,
		nodes: make([]*node, s),
	}
	for i := 0; i < len(h.nodes); i++ {
		h.nodes[i] = &node{
			register: make(map[string]labels.Set),
		}
	}
	return h
}

func (h *hashTable) Insert(k, n string, l map[string]string) uint32 {
	hidx := h.hash(k)
	return h.insert(hidx, k, n, l)
}

func (h *hashTable) Delete(k, n string, l map[string]string) {
	hidx := h.hash(k)
	h.delete(0, hidx, k, n, l)
}

func (h *hashTable) GetAllocated() (uint32, []*string) {
	used := make([]*string, 0)
	allocated := uint32(0)
	for _, n := range h.nodes {
		if n.key != "" {
			allocated++
			used = append(used, &n.key)
		}
	}
	return allocated, used
}

func (h *hashTable) insert(hidx uint32, k, n string, l map[string]string) uint32 {
	mergedlabel := labels.Merge(labels.Set(l), nil)
	// if entry is empty or the key is already used, insert the key and return the hash index
	if h.nodes[hidx].key == "" || h.nodes[hidx].key == k {
		// initialize
		if h.nodes[hidx].key == "" {
			h.nodes[hidx] = &node{
				key:      k,
				register: make(map[string]labels.Set),
			}
		}
		h.nodes[hidx].register[n] = mergedlabel
		return hidx
	}
	hidx++
	if hidx >= h.size {
		hidx = 0
	}
	return h.insert(hidx, k, n, l)
}

// k is the hashkey
// n is the name of the register or allocation
// l is the label
// ofidx is the overflow idx, used to ensure if we delete a resource that does not exist we stop
// hidx is the hash idx and we use overflow mapping by incrementing the hidx if a hash collision occurs
func (h *hashTable) delete(ofidx, hidx uint32, k, n string, l map[string]string) {
	// if entry is empty, insert the key and return the hash index
	if h.nodes[hidx].key == k {
		delete(h.nodes[hidx].register, n)
		// the hash entry has no longer has registers/allocations, so we can delete the key
		if len(h.nodes[hidx].register) == 0 {
			h.nodes[hidx] = &node{
				register: make(map[string]labels.Set),
			}
		}

		return
	}
	hidx++
	ofidx++
	if hidx >= h.size {
		hidx = 0
	}
	if ofidx == h.size {
		// the entry was not found, so we can stop
		return
	}
	h.delete(ofidx, hidx, k, n, l)
}

// hash
func (h *hashTable) hash(key string) uint32 {
	sum := 0
	for _, v := range key {
		sum += int(v)
	}
	return uint32(sum) % h.size
}
