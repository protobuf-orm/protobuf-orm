package graph_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/stretchr/testify/require"
)

func TestEntityMetadata(t *testing.T) {
	WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal("User", entity.Name())
		x.Equal("library.User", string(entity.FullName()))
		x.Equal("library", entity.Package())
		x.Equal("library/user.proto", entity.Path())
		x.NotNil(entity.Descriptor())
		x.Equal("User", string(entity.Descriptor().Name()))
		x.True(entity.HasFields())
		x.True(entity.HasEdges())
		x.True(entity.HasIndexes())
		x.True(entity.HasProps())
		x.True(entity.HasElems())
	})(t)
}

func TestProtoType(t *testing.T) {
	x := require.New(t)
	pt := graph.ProtoType("google.protobuf.Timestamp", "google/protobuf/timestamp.proto")
	x.Equal("google.protobuf.Timestamp", pt.ProtoType())
	x.Equal("google/protobuf/timestamp.proto", pt.ImportPath())

	prim := graph.ProtoType("string", "")
	x.Equal("string", prim.ProtoType())
	x.Equal("", prim.ImportPath())
}
