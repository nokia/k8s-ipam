package db

import (
	"fmt"

	"golang.org/x/exp/constraints"
	"k8s.io/apimachinery/pkg/labels"
)

type Entry[T constraints.Integer] interface {
	ID() T
	Labels() labels.Set
	String() string
}

type entry[T constraints.Integer] struct {
	id     T
	labels labels.Set
}
type Entries[T constraints.Integer] []Entry[T]

func (v entry[T]) ID() T              { return v.id }
func (v entry[T]) Labels() labels.Set { return v.labels }
func (v entry[T]) String() string     { return fmt.Sprintf("%v %s", v.ID(), v.Labels().String()) }

func NewEntry[T constraints.Integer](id T, l map[string]string) Entry[T] {
	var label labels.Set

	if l == nil {
		label = labels.Set{}
	} else {
		label = labels.Set(l)
	}
	return entry[T]{
		id:     id,
		labels: label,
	}
}
