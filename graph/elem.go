package graph

type Elem interface {
	Entity() Entity

	Name() string

	IsUnique() bool
	IsImmutable() bool
}
