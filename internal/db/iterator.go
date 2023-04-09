package db

import (
	"golang.org/x/exp/constraints"
)

type Iterator[T constraints.Integer] struct {
	current int
	keys    []T
	db      map[T]Entry[T]
}

func (r *Iterator[T]) Value() Entry[T] {
	return r.db[r.keys[r.current]]
}

func (r *Iterator[T]) Next() bool {
	r.current++
	return r.current < len(r.keys)
}

func (r *Iterator[T]) IsConsecutive() bool {
	//fmt.Println("id:", r.current, "prevId", r.keys[r.current-1], "currId", r.keys[r.current]-1)
	return r.keys[r.current-1] == r.keys[r.current]-1
}
