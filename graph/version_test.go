package graph_test

import (
	"context"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
)

func TestVersionField(t *testing.T) {
	t.Run("valid version field is detected", WithEntity(graphtest.File_graphtest_version_proto, "VersionField", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.HasVersionField())
		v := entity.GetVersionField()
		x.NotNil(v)
		x.Equal("updated_at", v.Name())
		x.True(v.IsVersion())
		x.Equal(ormpb.Type_TYPE_TIME, v.Type())
	}))
	t.Run("entity without a version field", WithEntity(graphtest.File_graphtest_entity_proto, "EntityEnabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.HasVersionField())
		x.Nil(entity.GetVersionField())
	}))
	t.Run("version field must be a time type", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_version_not_time_proto)
		x.Error(err)
		x.ErrorContains(err, "only the time type supports versioning")
	}))
	t.Run("version field cannot be unique", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_version_with_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "version field cannot be unique, nullable or immutable")
	}))

	// The key is marked unique and immutable by a normalization that runs after
	// every prop is parsed, so the check above cannot see it and this one has to
	// read the raw bit. Left open, the version became immutable, dropped out of
	// the PatchRequest -- which omits immutable props -- and the lock vanished
	// from the RPC without a word.
	t.Run("version field cannot be the key", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_version_with_key_proto)
		x.Error(err)
		x.ErrorContains(err, "version field cannot be the key")
	}))

	t.Run("there can be only one version field", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_version_many_proto)
		x.Error(err)
		x.ErrorContains(err, "there can be only one version field")
	}))
}
