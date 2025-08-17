package graph

import (
	"fmt"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Field interface {
	Prop
	Shape() Shape
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

func (f *protoField) Shape() Shape {
	d := f.source
	if f.Type().IsScalar() {
		return ScalarShape{V: f.Type()}
	}

	if d.IsMap() {
		k := ormpb.DeduceType(d.MapKey())
		v := d.MapValue()
		t := ormpb.TypeFromKind(v.Kind())
		s := MapShape{K: k, V: t}
		if !t.IsScalar() {
			s.S = shapeMessage(v)
		}

		return s
	}

	return shapeMessage(d)
}

func shapeMessage(fd protoreflect.FieldDescriptor) MessageShape {
	var d protoreflect.Descriptor

	k := fd.Kind()
	switch k {
	case protoreflect.EnumKind:
		d = fd.Enum()
	case protoreflect.MessageKind:
		d = fd.Message()
	default:
		panic(fmt.Errorf("%s: unexpected kind of the field: %s", fd.FullName(), k))
	}

	return MessageShape{
		Filepath:   d.ParentFile().Path(),
		FullName:   d.FullName(),
		Descriptor: d,
	}
}
