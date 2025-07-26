package graph

import (
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Describes how does the field look like.
type Shape interface {
	_shape()
}

type tagShape struct{}

func (t tagShape) _shape() {}

type ScalarShape struct {
	V ormpb.Type

	tagShape
}

type MapShape struct {
	K ormpb.Type   // Must be any integral or string type.
	V ormpb.Type   // It can be any type.
	S MessageShape // It is available only when [V] is TYPE_ENUM or TYPE_JSON.

	tagShape
}

type MessageShape struct {
	Filepath string
	FullName protoreflect.FullName

	tagShape
}
