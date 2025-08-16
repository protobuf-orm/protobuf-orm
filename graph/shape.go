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

	// EnumDescriptor or MessageDescriptor
	// Do not use this field except for resolving import paths
	// or checking the value type is message or enum.
	//
	// I didn't want to expose the descriptor, but I don't know
	// how to easily resolve the import path to the generated type.
	// Maybe I could write out all the import paths for Go, C++, Java, ...
	// but I think that is not scalable.
	Descriptor protoreflect.Descriptor

	tagShape
}
