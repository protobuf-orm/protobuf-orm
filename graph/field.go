package graph

import (
	"fmt"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
)

type Field interface {
	Prop
	Type() ormpb.Type
	Shape() Shape
}

type protoField struct {
	protoProp
	opts *ormpb.FieldOptions
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

func (f *protoField) Type() ormpb.Type {
	return f.opts.GetType()
}

func (f *protoField) Shape() Shape {
	d := f.source
	if f.Type().IsScalar() {
		return ScalarShape{V: f.Type()}
	}

	if d.IsMap() {
		k, err := ormpb.DeduceType(d.MapKey(), ormpb.Type_TYPE_UNSPECIFIED)
		if err != nil {
			panic(fmt.Errorf("map key must be a scalar type: %w", err))
		}

		v := d.MapValue()
		t := ormpb.TypeFromKind(v.Kind())
		s := MapShape{K: k, V: t}
		if t.IsMessage() {
			s.S = MessageShape{
				Filepath: v.ParentFile().Path(),
				FullName: v.FullName(),
			}
		}

		return s
	}

	return MessageShape{
		Filepath: d.Message().ParentFile().Path(),
		FullName: d.Message().FullName(),
	}
}
