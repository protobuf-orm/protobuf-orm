package graph_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Parsing normalizes a key field to unique+immutable. That normalization must
// happen on a private copy of the options, never on the shared descriptor
// extension, otherwise Parse would corrupt global state.
func TestParseDoesNotMutateDescriptor(t *testing.T) {
	x := require.New(t)
	md := library.File_library_user_proto.Messages().ByName("User")
	id := md.Fields().ByNumber(1) // key field; proto sets neither unique nor immutable

	read := func() (uniq, immut bool) {
		of := proto.GetExtension(id.Options(), ormpb.E_Field).(*ormpb.FieldOptions)
		return of.GetUnique(), of.GetImmutable()
	}
	u0, i0 := read()
	x.False(u0)
	x.False(i0)

	g := graph.NewGraph()
	x.NoError(graph.Parse(t.Context(), g, library.File_library_user_proto))

	u1, i1 := read()
	x.False(u1, "Parse must not mutate the shared descriptor's unique option")
	x.False(i1, "Parse must not mutate the shared descriptor's immutable option")

	// ...while the parsed graph still reports the key as unique and immutable.
	key := g.Entities[md.FullName()].Key()
	x.True(key.IsUnique())
	x.True(key.IsImmutable())
}

func TestParseIsIdempotent(t *testing.T) {
	x := require.New(t)
	read := func() graph.Entity {
		g := graph.NewGraph()
		x.NoError(graph.Parse(t.Context(), g, library.File_library_user_proto))
		x.NoError(graph.Parse(t.Context(), g, library.File_library_user_proto))
		md := library.File_library_user_proto.Messages().ByName("User")
		return g.Entities[md.FullName()]
	}
	e := read()
	x.NotNil(e)
	x.True(e.Key().IsUnique())
	x.True(e.Key().IsImmutable())
}

// A failed Parse must not leave any entity behind in the caller's graph.
func TestParseFailureLeavesGraphClean(t *testing.T) {
	x := require.New(t)
	g := graph.NewGraph()
	err := graph.Parse(t.Context(), g, graphtest.File_graphtest_invalid_key_not_exist_proto)
	x.Error(err)
	x.Empty(g.Entities)

	// A subsequent successful Parse into the same graph still works.
	x.NoError(graph.Parse(t.Context(), g, graphtest.File_graphtest_entity_proto))
	x.Contains(g.Entities, protoreflect.FullName("graphtest.EntityEnabled"))
}
