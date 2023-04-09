package db

import (
	"fmt"
	"sort"
	"sync"

	"golang.org/x/exp/constraints"
	"k8s.io/apimachinery/pkg/labels"
)

type DB[T constraints.Integer] interface {
	Set(e Entry[T]) error
	Get(id T) (Entry[T], error)
	GetByLabel(selector labels.Selector) Entries[T]
	GetAll() Entries[T]
	Has(id T) bool
	Delete(id T) error
	Count() int
	Iterate() *Iterator[T]
	IterateFree() *Iterator[T]

	FindFree() (Entry[T], error)
	FindFreeID(id T) (Entry[T], error)
	FindFreeRange(min, size T) (Entries[T], error)
	FindFreeSize(size T) (Entries[T], error)
}

type DBConfig[T constraints.Integer] struct {
	MaxEntries       uint64
	InitEntries      Entries[T]
	SetValidation    ValidationFn[T]
	DeleteValidation ValidationFn[T]
}

type ValidationFn[T constraints.Integer] func(id T) error

func NewDB[T constraints.Integer](c *DBConfig[T]) DB[T] {

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

type db[T constraints.Integer] struct {
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
	r.m.RLock()
	defer r.m.RUnlock()

	keys := make([]T, 0, len(r.store))
	for key := range r.store {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		return keys[i] < keys[j]
	})

	return &Iterator[T]{current: -1, keys: keys, db: r.store}
}

// IterateFree provides a list of keys and entries that
// are not allocated
func (r *db[T]) IterateFree() *Iterator[T] {
	r.m.RLock()
	defer r.m.RUnlock()

	var keys []T
	store := map[T]Entry[T]{}

	for id := 0; id < int(r.cfg.MaxEntries); id++ {
		_, exists := r.store[T(id)]
		if !exists {
			keys = append(keys, T(id))
			store[T(id)] = NewEntry(T(id), map[string]string{})
		}
	}
	sort.Slice(keys, func(i int, j int) bool {
		return keys[i] < keys[j]
	})

	//fmt.Println("keys", keys, "db", store)

	return &Iterator[T]{current: -1, keys: keys, db: store}
}

func (r *db[T]) FindFree() (Entry[T], error) {
	free := r.IterateFree()

	if free.Next() {
		return free.Value(), nil
	}
	return nil, fmt.Errorf("no free entry found")
}

func (r *db[T]) FindFreeID(id T) (Entry[T], error) {
	// validation
	if id > T(r.cfg.MaxEntries-1) {
		return nil, fmt.Errorf("id %d is bigger then max allowed entries: %d", id, r.cfg.MaxEntries-1)
	}
	if _, ok := r.store[id]; !ok {
		//free
		return NewEntry(T(id), map[string]string{}), nil
	}
	return nil, fmt.Errorf("id in use")
}

func (r *db[T]) FindFreeRange(start, size T) (Entries[T], error) {

	end := start + size - 1
	// validation
	if start >= T(r.cfg.MaxEntries-1) {
		return nil, fmt.Errorf("start %d is bigger then max allowed entries: %d", start, r.cfg.MaxEntries-1)
	}
	if end >= T(r.cfg.MaxEntries-1) {
		return nil, fmt.Errorf("end %d is bigger then max allowed entries: %d", end, r.cfg.MaxEntries-1)
	}

	entries := Entries[T]{}
	free := r.IterateFree()
	for free.Next() {
		if free.Value().ID() < start {
			continue
		}
		switch {
		case free.Value().ID() == start:
			entries = append(entries, free.Value())
		case free.Value().ID() > start && free.Value().ID() <= end:
			if !free.IsConsecutive() {
				return nil, fmt.Errorf("entry %d in use in range: start: %d, end %d", free.Value().ID(), start, end)
			}
			entries = append(entries, free.Value())
		default:
			return entries, nil
		}
	}
	return Entries[T]{}, nil
}

func (r *db[T]) FindFreeSize(size T) (Entries[T], error) {
	if size >= T(r.cfg.MaxEntries-1) {
		return nil, fmt.Errorf("size %d is bigger then max allowed entries: %d", size, r.cfg.MaxEntries-1)
	}
	entries := Entries[T]{}
	free := r.IterateFree()
	i := (T)(0)
	for free.Next() {
		i++
		fmt.Println(i, free.Value().ID())
		entries = append(entries, free.Value())
		if i >= size {
			return entries, nil
		}
	}

	return nil, fmt.Errorf("could not find free entries that fit in size %d", size)
}
