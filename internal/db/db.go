package db

import (
	"fmt"
	"sort"
	"sync"

	"golang.org/x/exp/constraints"
	"k8s.io/apimachinery/pkg/labels"
)

type DB[T constraints.Ordered] interface {
	Set(e Entry[T]) error
	Get(id T) (Entry[T], error)
	GetByLabel(selector labels.Selector) Entries[T]
	GetAll() Entries[T]
	Has(id T) bool
	Delete(id T) error
	Count() int
	Iterate() *Iterator[T]
}

type DBConfig[T constraints.Ordered] struct {
	MaxEntries       uint64
	InitEntries      Entries[T]
	SetValidation    ValidationFn[T]
	DeleteValidation ValidationFn[T]
}

type InitFn[T constraints.Ordered] func(e Entries[T])
type ValidationFn[T constraints.Ordered] func(id T) error

func NewDB[T constraints.Ordered](c *DBConfig[T]) DB[T] {
	r := &db[T]{
		m:     &sync.RWMutex{},
		store: make(map[T]Entry[T]),
		cfg:   c,
	}

	if r.cfg.InitEntries != nil {
		for _, e := range r.cfg.InitEntries {
			r.add(e)
		}
	}

	return r
}

type db[T constraints.Ordered] struct {
	m     *sync.RWMutex
	store map[T]Entry[T]
	cfg   *DBConfig[T]
}

func (r *db[T]) Set(e Entry[T]) error {
	r.m.Lock()
	defer r.m.Unlock()

	if r.cfg.SetValidation != nil {
		if err := r.cfg.SetValidation(e.ID()); err != nil {
			return err
		}
	}
	return r.add(e)
}

func (r *db[T]) add(e Entry[T]) error {
	r.store[e.ID()] = e
	return nil
}

func (r *db[T]) Get(id T) (Entry[T], error) {
	r.m.RLock()
	defer r.m.RUnlock()

	entry, ok := r.store[id]
	if !ok {
		return nil, fmt.Errorf("no match found for: %v", id)
	}
	return entry, nil
}

func (r *db[T]) GetByLabel(selector labels.Selector) Entries[T] {
	r.m.RLock()
	defer r.m.RUnlock()

	entries := Entries[T]{}

	iter := r.Iterate()
	for iter.Next() {
		if selector.Matches(iter.Value().Labels()) {
			entries = append(entries, iter.Value())
		}
	}
	return entries
}

func (r *db[T]) GetAll() Entries[T] {
	r.m.RLock()
	defer r.m.RUnlock()

	entries := Entries[T]{}

	iter := r.Iterate()
	for iter.Next() {
		entries = append(entries, iter.Value())
	}
	return entries
}

func (r *db[T]) Has(id T) bool {
	r.m.RLock()
	defer r.m.RUnlock()
	_, ok := r.store[id]
	return ok
}

func (r *db[T]) Delete(id T) error {
	r.m.Lock()
	defer r.m.Unlock()

	if r.cfg.DeleteValidation != nil {
		if err := r.cfg.DeleteValidation(id); err != nil {
			return err
		}
	}
	delete(r.store, id)
	return nil
}

func (r *db[T]) Count() int {
	r.m.RLock()
	defer r.m.RUnlock()

	return len(r.store)
}

func (r *db[T]) Iterate() *Iterator[T] {
	keys := make([]T, 0, len(r.store))
	for key := range r.store {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		return keys[i] < keys[j]
	})

	fmt.Println(keys)
	return &Iterator[T]{current: -1, keys: keys, db: r.store}
}
