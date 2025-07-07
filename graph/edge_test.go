package graph_test

import (
	"context"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/internal/iters"
	"github.com/stretchr/testify/require"
)

func TestEdgeO2M(t *testing.T) {
	t.Run("valid", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		parent, ok := iters.Find(entity.Edges(), func(v graph.Edge) bool { return v.FullName().Name() == "parent" })
		x.True(ok)
		children, ok := iters.Find(entity.Edges(), func(v graph.Edge) bool { return v.FullName().Name() == "children" })
		x.True(ok)

		x.False(parent.IsUnique())
		x.False(children.IsUnique())
	}))
	t.Run("parent marked as unique explicitly", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_o2m_parent_marked_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "children: back reference is unique edge so it cannot have repeated cardinality")
	}))
	t.Run("children marked as unique explicitly", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_o2m_children_marked_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "children: edge with repeated cardinality cannot be unique")
	}))
	t.Run("reference marked as unique explicitly", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_o2m_ref_marked_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "children: edge with repeated cardinality cannot be unique")
	}))
}
