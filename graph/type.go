package graph

import (
	"fmt"
	"strings"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func GoTypeOf(p Prop, f func(v protogen.GoIdent) string) string {
	return GoType(p.Descriptor(), p.Type(), f)
}

func GoType(d protoreflect.FieldDescriptor, t ormpb.Type, f func(v protogen.GoIdent) string) string {
	switch d.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.EnumKind:
		d := d.Enum()
		pkg := MustGetGoImportPath(d.ParentFile())
		return f(pkg.Ident(string(d.Name())))
	case protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		if t == ormpb.Type_TYPE_UUID {
			return f(protogen.GoImportPath("github.com/google/uuid").Ident("UUID"))
		} else {
			return "[]byte"
		}
	case protoreflect.MessageKind:
		switch {
		case d.IsMap():
			// Key must be scalar so t and f are not needed.
			k := GoType(d.MapKey(), ormpb.Type_TYPE_UNSPECIFIED, nil)
			v := GoType(d.MapValue(), t, f)
			return fmt.Sprintf("map[%s]%s", k, v)

		case t == ormpb.Type_TYPE_TIME:
			return f(protogen.GoImportPath("time").Ident("Time"))

		default:
			d := d.Message()
			name, ok := strings.CutPrefix(string(d.FullName()), string(d.ParentFile().Package()))
			if ok {
				name = name[1:]
			}
			name = strings.ReplaceAll(name, ".", "_")

			pkg := MustGetGoImportPath(d.ParentFile())
			return f(pkg.Ident(name))
		}

	case protoreflect.GroupKind:
		panic("not implemented: group")
	default:
		panic("unknown field kind")
	}
}

func GetGoImportPath(d protoreflect.FileDescriptor) (protogen.GoImportPath, bool) {
	opts := d.Options().(*descriptorpb.FileOptions)
	v := opts.GetGoPackage()
	if v == "" {
		return "", false
	}

	es := strings.SplitN(v, ";", 2)
	return protogen.GoImportPath(es[0]), true
}

func MustGetGoImportPath(d protoreflect.FileDescriptor) protogen.GoImportPath {
	v, ok := GetGoImportPath(d.ParentFile())
	if !ok {
		panic(fmt.Sprintf("Go import path for %s not found", d.FullName()))
	}

	return v
}
