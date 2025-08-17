package graph_test

import (
	"context"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/stretchr/testify/require"
)

func TestPropValidity(t *testing.T) {
	t.Run("prop must be either field or edge but not both", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_prop_both_type_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.PropBothType.alias: only one of")
		x.ErrorContains(err, "orm.field")
		x.ErrorContains(err, "orm.edge")
	}))
	t.Run("field cannot be a message type", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_prop_meesage_type_field_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.PropMessageTypeField.alias: field cannot be a message type")
	}))
	t.Run("edge must target an entity", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_prop_edge_to_non_entity_proto)
		x.Error(err)
		x.ErrorContains(err, "graphtest.PropEdgeToNonEntity.entity: target is not an entity")
	}))
}
