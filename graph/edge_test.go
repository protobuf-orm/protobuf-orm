package graph_test

import (
	"context"
	"slices"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/internal/iters"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
)

func TestEdgeO2O(t *testing.T) {
	t.Run("self referencing", WithEntity(graphtest.File_graphtest_edge_o2o_self_ref_proto, "SelfRef", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		edges := slices.Collect(entity.Edges())
		x.Len(edges, 1)

		target := edges[0].Entity()
		x.Same(entity, target)
	}))
	t.Run("circular reference", WithEntity(graphtest.File_graphtest_edge_o2o_circular_ref_proto, "A", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		edges := slices.Collect(entity.Edges())
		x.Len(edges, 1)

		b := edges[0].Entity()

		edges = slices.Collect(b.Edges())
		x.Len(edges, 1)

		a := edges[0].Entity()
		x.Same(entity, a)
	}))
}

func TestEdgeO2OBackReference(t *testing.T) {
	t.Run("from: makes both edges unique", WithEntity(graphtest.File_graphtest_edge_o2o_proto, "Node", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		next, ok := iters.Find(entity.Edges(), func(v graph.Edge) bool { return v.FullName().Name() == "next" })
		x.True(ok)
		prev, ok := iters.Find(entity.Edges(), func(v graph.Edge) bool { return v.FullName().Name() == "prev" })
		x.True(ok)
		// prev declares next as its back-reference, so they are a one-to-one
		// pair and the normalization (applied on successful parse) marks both
		// unique.
		x.True(next.IsUnique())
		x.True(prev.IsUnique())
		x.Equal(next.FullName(), prev.Inverse().FullName())
		x.Equal(prev.FullName(), next.Reverse().FullName())
	}))
}

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
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_o2m_parent_marked_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "children: back reference is unique edge so it cannot have repeated cardinality")
	}))
	t.Run("children marked as unique explicitly", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_o2m_children_marked_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "children: edge with repeated cardinality cannot be unique")
	}))
	t.Run("reference marked as unique explicitly", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_o2m_ref_marked_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "children: edge with repeated cardinality cannot be unique")
	}))
}

func TestEdgeOnDelete(t *testing.T) {
	onDelete := func(entity graph.Entity) map[string]ormpb.OnDelete {
		got := map[string]ormpb.OnDelete{}
		for e := range entity.Edges() {
			got[e.Name()] = e.OnDelete()
		}
		return got
	}

	t.Run("each value", WithEntity(graphtest.File_graphtest_edge_on_delete_proto, "OnDelete", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal(map[string]ormpb.OnDelete{
			"unspecified": ormpb.OnDelete_ON_DELETE_UNSPECIFIED,
			"restrict":    ormpb.OnDelete_ON_DELETE_RESTRICT,
			"cascade":     ormpb.OnDelete_ON_DELETE_CASCADE,
			"set_null":    ormpb.OnDelete_ON_DELETE_SET_NULL,
		}, onDelete(entity))
	}))
	t.Run("an edge that says nothing", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		for e := range entity.Edges() {
			x.Equal(ormpb.OnDelete_ON_DELETE_UNSPECIFIED, e.OnDelete(), e.Name())
		}
	}))

	// The shapes an edge that holds the key comes in. The first is "each
	// value" above: a single edge with no inverse and no reverse.
	t.Run("single, no inverse, its reverse a list", WithEntity(graphtest.File_graphtest_edge_on_delete_proto, "OnDeleteTree", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal(map[string]ormpb.OnDelete{
			"parent":   ormpb.OnDelete_ON_DELETE_CASCADE,
			"children": ormpb.OnDelete_ON_DELETE_UNSPECIFIED,
		}, onDelete(entity))
	}))
	t.Run("single, written from a list", WithEntity(graphtest.File_graphtest_edge_on_delete_proto, "OnDeleteGroup", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal(map[string]ormpb.OnDelete{
			"members": ormpb.OnDelete_ON_DELETE_UNSPECIFIED,
			"group":   ormpb.OnDelete_ON_DELETE_SET_NULL,
		}, onDelete(entity))
	}))
	t.Run("single, written from a single edge", WithEntity(graphtest.File_graphtest_edge_on_delete_proto, "OnDeleteNode", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal(map[string]ormpb.OnDelete{
			"next": ormpb.OnDelete_ON_DELETE_UNSPECIFIED,
			"prev": ormpb.OnDelete_ON_DELETE_RESTRICT,
		}, onDelete(entity))
	}))

	t.Run("not on a list", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_edge_on_delete_list_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.EdgeOnDeleteList.items: on_delete is said on the edge that holds the key, and a list holds none")
		x.Empty(g.Entities)
	}))
	t.Run("not on a list written from a single edge", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_edge_on_delete_from_list_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.EdgeOnDeleteFromList.children: on_delete is said on the edge that holds the key, and a list holds none")
	}))
	t.Run("not on the To side of a one-to-one", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_edge_on_delete_o2o_to_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.EdgeOnDeleteO2oTo.next: on_delete is said on the edge that holds the key, and of a one-to-one that is the side written from this one, graphtest.EdgeOnDeleteO2oTo.prev; say it there")
	}))
	t.Run("nor across two entities", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_edge_on_delete_o2o_to_other_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.EdgeOnDeletePerson.card: on_delete is said on the edge that holds the key, and of a one-to-one that is the side written from this one, graphtest.EdgeOnDeleteCard.owner; say it there")
		x.Empty(g.Entities)
	}))
	t.Run("set null on an edge that is not nullable", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_edge_on_delete_set_null_not_nullable_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.EdgeOnDeleteSetNullNotNullable.parent: on_delete ON_DELETE_SET_NULL clears the edge, and this one is not nullable")
	}))
}

// A schema does not mean something else by the order it is written in: `from`
// finds the edge it names wherever each is declared.
func TestEdgeFromOrder(t *testing.T) {
	inverse := func(x *require.Assertions, entity graph.Entity, edge string) graph.Edge {
		e, ok := iters.Find(entity.Edges(), func(v graph.Edge) bool { return v.Name() == edge })
		x.True(ok)
		return e.Inverse()
	}

	t.Run("a list written from an edge declared after it", WithEntity(graphtest.File_graphtest_edge_from_order_proto, "FromFirstSelf", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal("graphtest.FromFirstSelf.parent", string(inverse(x, entity, "children").FullName()))
	}))
	t.Run("across two entities, the one written from first", WithEntity(graphtest.File_graphtest_edge_from_order_proto, "FromFirstMember", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal("graphtest.FromFirstGroup.members", string(inverse(x, entity, "group").FullName()))
	}))
	t.Run("across two entities, the one it names first", WithEntity(graphtest.File_graphtest_edge_from_order_proto, "NamedFirstMember", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal("graphtest.NamedFirstGroup.members", string(inverse(x, entity, "group").FullName()))

		group := g.Entities["graphtest.NamedFirstGroup"]
		members, ok := iters.Find(group.Edges(), func(v graph.Edge) bool { return v.Name() == "members" })
		x.True(ok)
		x.Equal("graphtest.NamedFirstMember.group", string(members.Reverse().FullName()))
	}))
	t.Run("a from that names no edge is still refused", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_edge_from_not_found_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.EdgeFromNotFound.children: back reference not found on the target entity: graphtest.EdgeFromNotFound[9]")
		x.Empty(g.Entities)
	}))
}
