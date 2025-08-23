package graph

import (
	"github.com/protobuf-orm/protobuf-orm/ormpb"
)

type Field interface {
	Prop
	isField()
}

type protoField struct {
	protoProp
	opts *ormpb.FieldOptions
}

func (f *protoField) Type() ormpb.Type {
	return f.opts.GetType()
}

func (f *protoField) IsNullable() bool {
	if f.isRepeated() {
		// There is no way to distinguish between empty and null in proto.
		return false
	}
	return f.opts.GetNullable() ||
		f.source.HasOptionalKeyword() ||
		f.source.HasPresence()
}

func (f *protoField) IsOptional() bool {
	if f.isRepeated() {
		// Empty input for repeated prop is treated as an empty list or map.
		return true
	}
	return f.IsNullable() ||
		f.HasDefault()
}

func (f *protoField) isField() {}
