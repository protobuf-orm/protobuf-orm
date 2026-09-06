// Package gogen is the part of a graph that only a Go code generator needs.
//
// It is a package of its own because of what it imports. `protogen` is
// protobuf's plugin library, and it carries Go's own compiler front end with it
// -- `go/types`, `go/parser`, `go/ast`, `go/printer`, `go/constant`. That is
// right for a plugin, which reads Go and writes Go, and wrong for everything
// else: `graph` describes entities at run time, generated code refers to
// `graph.Edge`, and so every server built from that code linked a Go parser it
// never called.
//
// Measured on one app compiled to `GOOS=js GOARCH=wasm`, that was 3.5 MB of the
// module a browser had to download, and the same weight sits in every process
// binary. Nothing reported it; a linked package that is never called has no
// symptom but its size.
//
// So the split is by what a caller is. A generator imports this; a program that
// merely has a graph imports `graph` and gets no compiler.
package gogen

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
)

// ParseFiles parses every file marked for generation, in order, into g. It is
// the entry point for a protoc/buf plugin, which passes gen.Files. Files not
// marked for generation (imports) are skipped.
func ParseFiles(ctx context.Context, g *graph.Graph, fs []*protogen.File) error {
	for _, f := range fs {
		if !f.Generate {
			continue
		}

		d := f.Desc
		if err := graph.Parse(ctx, g, d); err != nil {
			return fmt.Errorf("%s: %w", d.Path(), err)
		}
	}

	return nil
}

func GoTypeOf(p graph.Prop, f func(v protogen.GoIdent) string) string {
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
			return f(protogen.GoImportPath("uuid").Ident("UUID"))
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
			name := string(d.FullName())
			if pkg := string(d.ParentFile().Package()); pkg != "" {
				name = strings.TrimPrefix(name, pkg+".")
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
	v, ok := GetGoImportPath(d)
	if !ok {
		panic(fmt.Sprintf("Go import path for %s not found", d.FullName()))
	}

	return v
}
