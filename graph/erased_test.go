package graph_test

import (
	"context"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// refuses is a file that must not parse, and the words the refusal has to
// carry -- the message is what a schema author reads, so it is part of the
// contract rather than an implementation detail.
func refuses(d protoreflect.FileDescriptor, says string) func(t *testing.T) {
	return WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, d)
		x.Error(err)
		x.ErrorContains(err, says)
	})
}

func TestErasedField(t *testing.T) {
	t.Run("valid erased field is detected", WithEntity(graphtest.File_graphtest_erased_proto, "ErasedField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.HasErasedField())

		v := entity.GetErasedField()
		x.NotNil(v)
		x.Equal("erased_at", v.Name())
		x.True(v.IsErased())
		x.Equal(ormpb.Type_TYPE_TIME, v.Type())

		// Being null is how it says the row is still there, and it is that
		// whether or not the schema troubled to say so.
		x.True(v.IsNullable())

		// And it is not the version field, which is the other thing a time
		// column of this shape could be.
		x.False(v.IsVersion())
	}))

	t.Run("entity without an erased field", WithEntity(graphtest.File_graphtest_entity_proto, "EntityEnabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.HasErasedField())
		x.Nil(entity.GetErasedField())
	}))

	// A row is erased by asking to erase it. A request that could assign the
	// date would be a second way to do it, one that skips whatever Erase does
	// besides writing the column -- and clearing the date would be a delete
	// undone, which no RPC means.
	t.Run("it is not a prop a patch request carries", WithEntity(graphtest.File_graphtest_erased_proto, "ErasedField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		vs := []string{}
		for p := range graph.PatchProps(entity) {
			vs = append(vs, p.Name())
		}

		x.NotContains(vs, "erased_at")
		x.Contains(vs, "alias", "and the ordinary props are still there")
	}))

	t.Run("must be a time",
		refuses(graphtest.File_graphtest_invalid_erased_not_time_proto,
			"only the time type can say that a row was erased"))
	t.Run("cannot be the key",
		refuses(graphtest.File_graphtest_invalid_erased_with_key_proto,
			"erased field cannot be the key"))
	t.Run("cannot be unique",
		refuses(graphtest.File_graphtest_invalid_erased_with_unique_proto,
			"erased field cannot be unique or immutable"))
	t.Run("cannot be immutable",
		refuses(graphtest.File_graphtest_invalid_erased_with_immutable_proto,
			"erased field cannot be unique or immutable"))
	t.Run("cannot be told it is not nullable",
		refuses(graphtest.File_graphtest_invalid_erased_not_nullable_proto,
			"erased field is nullable"))
	t.Run("cannot also be the version field",
		refuses(graphtest.File_graphtest_invalid_erased_with_version_proto,
			"erased field cannot also be the version field"))
	t.Run("there can be only one",
		refuses(graphtest.File_graphtest_invalid_erased_many_proto,
			"there can be only one erased field"))
}

func TestIndexExcludesErased(t *testing.T) {
	// The default, and the whole reason there is one: a row that is gone should
	// not go on holding the name it had, or the alias of something erased could
	// never be used again.
	t.Run("a unique index of a soft-erasing entity is partial", WithEntity(graphtest.File_graphtest_erased_proto, "ErasedField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		vs := map[string]graph.Index{}
		for i := range entity.Indexes() {
			vs[i.Name()] = i
		}

		x.True(vs["alias"].ExcludesErased())

		// Unless it says otherwise, which is for a name meant to stay taken.
		x.False(vs["name"].ExcludesErased())
	}))

	t.Run("an entity that does not erase softly has nothing to exclude", WithEntity(graphtest.File_graphtest_entity_proto, "EntityEnabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		for i := range entity.Indexes() {
			x.False(i.ExcludesErased(), i.Name())
		}
	}))

	t.Run("saying it of an index that is not unique is refused",
		refuses(graphtest.File_graphtest_invalid_index_includes_erased_not_unique_proto,
			"includes_erased says nothing about an index that is not unique"))
	t.Run("saying it of an entity with no erased field is refused",
		refuses(graphtest.File_graphtest_invalid_index_includes_erased_wo_erased_proto,
			"includes_erased says nothing about an entity that has no erased field"))
}
