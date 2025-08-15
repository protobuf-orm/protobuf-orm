package graph_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestScalarField(t *testing.T) {
	const RANGE = protoreflect.FieldNumber(0x1F - 0x10 + 1)
	WithEntity(graphtest.File_graphtest_field_proto, "ScalarField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
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
				x.True(f.IsNullable())
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

func TestNonScalarField(t *testing.T) {
	WithEntity(graphtest.File_graphtest_field_proto, "NonScalarField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
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
			x.True(f.IsNullable())
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
	})
}
