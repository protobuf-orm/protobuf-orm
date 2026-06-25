package graph_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// goName resolves a GoIdent to its bare type name (ignoring the import path),
// which is enough to assert the type-mapping logic.
func goName(id protogen.GoIdent) string { return id.GoName }

func TestGoType(t *testing.T) {
	mapping := graphtest.File_graphtest_field_type_mapping_proto.Messages().ByName("FieldTypeMapping").Fields()
	value := graphtest.File_graphtest_field_proto.Messages().ByName("ValueField").Fields()
	maps := graphtest.File_graphtest_field_proto.Messages().ByName("MapField").Fields()
	user := library.File_library_user_proto.Messages().ByName("User").Fields()

	for _, tc := range []struct {
		name  string
		field protoreflect.FieldDescriptor
		typ   ormpb.Type
		want  string
	}{
		{"int32", mapping.ByName("v_i32"), ormpb.Type_TYPE_INT32, "int32"},
		{"uint32", mapping.ByName("v_u32"), ormpb.Type_TYPE_UINT32, "uint32"},
		{"int64", mapping.ByName("v_i64"), ormpb.Type_TYPE_INT64, "int64"},
		{"sint64", mapping.ByName("v_si64"), ormpb.Type_TYPE_SINT64, "int64"},
		{"float", mapping.ByName("v_f32"), ormpb.Type_TYPE_FLOAT, "float32"},
		{"double", mapping.ByName("v_f64"), ormpb.Type_TYPE_DOUBLE, "float64"},
		{"bool", mapping.ByName("v_bool"), ormpb.Type_TYPE_BOOL, "bool"},
		{"string", mapping.ByName("v_string"), ormpb.Type_TYPE_STRING, "string"},
		{"bytes", mapping.ByName("v_bytes"), ormpb.Type_TYPE_BYTES, "[]byte"},
		{"uuid", user.ByNumber(1), ormpb.Type_TYPE_UUID, "UUID"},
		{"time", mapping.ByName("wkt_time"), ormpb.Type_TYPE_TIME, "Time"},
		{"enum", value.ByName("implicit_enum"), ormpb.Type_TYPE_ENUM, "Level"},
		{"nested message", mapping.ByName("v_message"), ormpb.Type_TYPE_JSON, "FieldTypeMapping_Something"},
		{"map", maps.ByName("implicit_string"), ormpb.Type_TYPE_JSON, "map[string]string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, graph.GoType(tc.field, tc.typ, goName))
		})
	}
}

func TestGoTypeImportPath(t *testing.T) {
	x := require.New(t)
	id := library.File_library_user_proto.Messages().ByName("User").Fields().ByNumber(1)
	got := graph.GoType(id, ormpb.Type_TYPE_UUID, func(v protogen.GoIdent) string {
		return string(v.GoImportPath) + "." + v.GoName
	})
	x.Equal("github.com/google/uuid.UUID", got)
}

func TestIsCollection(t *testing.T) {
	WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		props := map[string]graph.Prop{}
		for p := range entity.Props() {
			props[p.Name()] = p
		}
		x.False(graph.IsCollection(props["id"]), "scalar field is not a collection")
		x.False(graph.IsCollection(props["name"]), "scalar field is not a collection")
		x.True(graph.IsCollection(props["labels"]), "map field is a collection")
		x.True(graph.IsCollection(props["children"]), "repeated edge is a collection")
		x.False(graph.IsCollection(props["parent"]), "single edge is not a collection")
	})(t)
}
