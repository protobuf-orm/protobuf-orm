package ormpb

import (
	"fmt"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

// DeduceType deduces proper type for the given field.
// It returns `want` as-is if the `want` can be a type for the given field.
// [Type_TYPE_MESSAGE] is returned only and only if the field type is a explicit message type and not known message.
// Known messages and their deduced types are as follows::
//
//	"google.protobuf.Timestamp": Type_TYPE_TIME
func DeduceType(f protoreflect.FieldDescriptor, want Type) (Type, error) {
	if want != Type_TYPE_UNSPECIFIED {
		// TODO: do validation
		// e.g. if the kind is Bool, the type must be Bool.
		return want, nil
	}

	v := Type_TYPE_UNSPECIFIED
	switch f.Kind() {
	case protoreflect.BoolKind:
		v = Type_TYPE_BOOL
	case protoreflect.EnumKind:
		v = Type_TYPE_ENUM
	case protoreflect.Int32Kind:
		v = Type_TYPE_INT32
	case protoreflect.Sint32Kind:
		v = Type_TYPE_SINT32
	case protoreflect.Uint32Kind:
		v = Type_TYPE_UINT32
	case protoreflect.Int64Kind:
		v = Type_TYPE_INT64
	case protoreflect.Sint64Kind:
		v = Type_TYPE_SINT64
	case protoreflect.Uint64Kind:
		v = Type_TYPE_UINT64
	case protoreflect.Sfixed32Kind:
		v = Type_TYPE_SFIXED32
	case protoreflect.Fixed32Kind:
		v = Type_TYPE_FIXED32
	case protoreflect.FloatKind:
		v = Type_TYPE_FLOAT
	case protoreflect.Sfixed64Kind:
		v = Type_TYPE_SFIXED64
	case protoreflect.Fixed64Kind:
		v = Type_TYPE_FIXED64
	case protoreflect.DoubleKind:
		v = Type_TYPE_DOUBLE
	case protoreflect.StringKind:
		v = Type_TYPE_STRING
	case protoreflect.BytesKind:
		v = Type_TYPE_BYTES
	case protoreflect.MessageKind:
		v = Type_TYPE_MESSAGE
	case protoreflect.GroupKind:
		v = Type_TYPE_GROUP

	default:
		return Type_TYPE_UNSPECIFIED, fmt.Errorf("unknown type of proto field: %v", f.Kind().String())
	}
	if v != Type_TYPE_MESSAGE {
		return v, nil
	}
	if f.IsMap() {
		return Type_TYPE_JSON, nil
	}
	switch f.Message().FullName() {
	case "google.protobuf.Timestamp":
		return Type_TYPE_TIME, nil
	}

	return v, nil
}

func (t Type) Decay() Type {
	switch t {
	case
		Type_TYPE_INT32,
		Type_TYPE_INT64,
		Type_TYPE_SINT32,
		Type_TYPE_SINT64,
		Type_TYPE_SFIXED32,
		Type_TYPE_SFIXED64:
		return Type_TYPE_INT
	case
		Type_TYPE_UINT32,
		Type_TYPE_UINT64,
		Type_TYPE_FIXED32,
		Type_TYPE_FIXED64:
		return Type_TYPE_UINT
	}

	return t
}
