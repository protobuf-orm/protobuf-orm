package graph

import "google.golang.org/protobuf/reflect/protoreflect"

type Elem interface {
	Entity() Entity

	Name() string
	Number() protoreflect.FieldNumber

	IsUnique() bool
	IsImmutable() bool
}
