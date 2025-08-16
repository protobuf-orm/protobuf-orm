package ormpb

import (
	"fmt"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

func TypeFromKind(k protoreflect.Kind) Type {
	v := Type_TYPE_UNSPECIFIED
	switch k {
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
	}

	return v
}

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

	v := TypeFromKind(f.Kind())
	if v == Type_TYPE_UNSPECIFIED {
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
	case "google.protobuf.Struct":
		return Type_TYPE_JSON, nil
	}

	return v, nil
}

func (t Type) IsMessage() bool {
	return t.Decay() == Type_TYPE_MESSAGE
}

func (t Type) IsScalar() bool {
	return !t.IsMessage()
}

func (t Type) Decay() Type {
	switch t {
	case Type_TYPE_FLOAT,
		Type_TYPE_DOUBLE:
		return Type_TYPE_FLOAT
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
	case
		Type_TYPE_ENUM,
		Type_TYPE_MESSAGE,
		Type_TYPE_JSON,
		Type_TYPE_TIME:
		return Type_TYPE_MESSAGE
	case
		Type_TYPE_UUID:
		return Type_TYPE_BYTES
	}

	return t
}
