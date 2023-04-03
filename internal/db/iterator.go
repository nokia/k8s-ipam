package db

import "golang.org/x/exp/constraints"

type Iterator[T constraints.Ordered] struct {
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
