package graph_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
)

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
