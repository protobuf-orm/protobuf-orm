package graph_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestFieldTypeMapping(t *testing.T) {
	WithEntity(graphtest.File_graphtest_field_type_mapping_proto, "FieldTypeMapping", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		fields := map[string]ormpb.Type{}
		for f := range entity.Fields() {
			fields[f.Name()] = f.Type()
		}

		x.Equal(ormpb.Type_TYPE_DOUBLE, fields["v_f64"])
		x.Equal(ormpb.Type_TYPE_FLOAT, fields["v_f32"])
		x.Equal(ormpb.Type_TYPE_INT32, fields["v_i32"])
		x.Equal(ormpb.Type_TYPE_INT64, fields["v_i64"])
		x.Equal(ormpb.Type_TYPE_UINT32, fields["v_u32"])
		x.Equal(ormpb.Type_TYPE_UINT64, fields["v_u64"])
		x.Equal(ormpb.Type_TYPE_SINT32, fields["v_si32"])
		x.Equal(ormpb.Type_TYPE_SINT64, fields["v_si64"])
		x.Equal(ormpb.Type_TYPE_FIXED32, fields["v_fi32"])
		x.Equal(ormpb.Type_TYPE_FIXED64, fields["v_fi64"])
		x.Equal(ormpb.Type_TYPE_SFIXED32, fields["v_sfi32"])
		x.Equal(ormpb.Type_TYPE_SFIXED64, fields["v_sfi64"])
		x.Equal(ormpb.Type_TYPE_BOOL, fields["v_bool"])
		x.Equal(ormpb.Type_TYPE_STRING, fields["v_string"])
		x.Equal(ormpb.Type_TYPE_BYTES, fields["v_bytes"])

		x.Equal(ormpb.Type_TYPE_TIME, fields["wkt_time"])
		x.Equal(ormpb.Type_TYPE_JSON, fields["wkt_struct"])
		x.Equal(ormpb.Type_TYPE_JSON, fields["wkt_value"])

		x.Equal(ormpb.Type_TYPE_JSON, fields["v_message"])
	})(t)
}

func TestValueField(t *testing.T) {
	const RANGE = protoreflect.FieldNumber(0x1F - 0x10 + 1)
	WithEntity(graphtest.File_graphtest_field_proto, "ValueField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		fields := map[protoreflect.FieldNumber]graph.Field{}
		for f := range entity.Fields() {
			fields[f.Number()] = f
		}

		t.Run("implicit fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0x10
				f := fields[j]
				x.False(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.False(f.IsOptional())
				x.False(f.IsNullable())
			}
		})
		t.Run("explicit fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0x30
				f := fields[j]
				x.False(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.True(f.IsOptional())
				x.True(f.IsNullable())
			}
		})
		t.Run("repeated fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0x50
				f := fields[j]
				x.False(f.HasDefault())
				x.True(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.True(f.IsOptional())
				x.False(f.IsNullable())
			}
		})
		t.Run("nullable fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0x70
				f := fields[j]
				x.False(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.True(f.IsOptional())
				x.True(f.IsNullable())
			}
		})
		t.Run("implicit fields with default value", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0x90
				f := fields[j]
				x.True(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.True(f.IsOptional())
				x.False(f.IsNullable())
			}
		})
		t.Run("explicit fields with default value", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0xB0
				f := fields[j]
				x.True(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.True(f.IsOptional())
				x.True(f.IsNullable())
			}
		})
		t.Run("implicit immutable fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0xD0
				f := fields[j]
				x.False(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.True(f.IsImmutable())
				x.False(f.IsOptional())
				x.False(f.IsNullable())
			}
		})
		t.Run("explicit immutable fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0xF0
				f := fields[j]
				x.False(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.True(f.IsImmutable())
				x.True(f.IsOptional())
				x.True(f.IsNullable())
			}
		})
	})(t)
}

func TestMessageField(t *testing.T) {
	WithEntity(graphtest.File_graphtest_field_proto, "MessageField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		fields := map[protoreflect.FieldNumber]graph.Field{}
		for f := range entity.Fields() {
			fields[f.Number()] = f
		}

		t.Run("explicit fields", func(t *testing.T) {
			x := require.New(t)

			f := fields[0x30]
			x.False(f.HasDefault())
			x.False(f.IsList())
			x.False(f.IsUnique())
			x.False(f.IsImmutable())
			x.True(f.IsOptional())
			x.True(f.IsNullable())
		})
		t.Run("repeated fields", func(t *testing.T) {
			x := require.New(t)

			f := fields[0x50]
			x.False(f.HasDefault())
			x.True(f.IsList())
			x.False(f.IsUnique())
			x.False(f.IsImmutable())
			x.True(f.IsOptional())
			x.False(f.IsNullable())
		})
		t.Run("nullable fields", func(t *testing.T) {
			x := require.New(t)

			f := fields[0x70]
			x.False(f.HasDefault())
			x.False(f.IsList())
			x.False(f.IsUnique())
			x.False(f.IsImmutable())
			x.True(f.IsOptional())
			x.True(f.IsNullable())
		})
		t.Run("explicit fields with default value", func(t *testing.T) {
			x := require.New(t)

			f := fields[0xB0]
			x.True(f.HasDefault())
			x.False(f.IsList())
			x.False(f.IsUnique())
			x.False(f.IsImmutable())
			x.True(f.IsOptional())
			x.True(f.IsNullable())
		})
		t.Run("explicit immutable fields", func(t *testing.T) {
			x := require.New(t)

			f := fields[0xF0]
			x.False(f.HasDefault())
			x.False(f.IsList())
			x.False(f.IsUnique())
			x.True(f.IsImmutable())
			x.True(f.IsOptional())
			x.True(f.IsNullable())
		})
	})(t)
}

func TestMapField(t *testing.T) {
	const RANGE = protoreflect.FieldNumber(0x12 - 0x10 + 1)
	WithEntity(graphtest.File_graphtest_field_proto, "MapField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		fields := map[protoreflect.FieldNumber]graph.Field{}
		for f := range entity.Fields() {
			fields[f.Number()] = f
		}

		t.Run("implicit fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0x10
				f := fields[j]
				x.False(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.True(f.IsOptional())
				x.False(f.IsNullable())
			}
		})
		t.Run("implicit fields with default value", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0x90
				f := fields[j]
				x.True(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.False(f.IsImmutable())
				x.True(f.IsOptional())
				x.False(f.IsNullable())
			}
		})
		t.Run("implicit immutable fields", func(t *testing.T) {
			x := require.New(t)
			for i := range RANGE {
				j := i + 0xD0
				f := fields[j]
				x.False(f.HasDefault())
				x.False(f.IsList())
				x.False(f.IsUnique())
				x.True(f.IsImmutable())
				x.True(f.IsOptional())
				x.False(f.IsNullable())
			}
		})
	})(t)
}
