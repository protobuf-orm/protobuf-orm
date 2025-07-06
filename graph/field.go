package graph

import "github.com/protobuf-orm/protobuf-orm/ormpb"

type Field interface {
	Prop
	Type() ormpb.Type
}

type protoField struct {
	protoProp
	opts *ormpb.FieldOptions
}

func (f *protoField) Type() ormpb.Type {
	return f.opts.GetType()
}

func (f *protoField) IsUnique() bool {
	return f.opts.GetUnique()
}

func (f *protoField) IsNullable() bool {
	return f.opts.GetNullable()
}

func (f *protoField) IsImmutable() bool {
	return f.opts.GetImmutable()
}
