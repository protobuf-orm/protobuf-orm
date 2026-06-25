package graph_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestIndexValidity(t *testing.T) {
	t.Run("valid composite index resolves props in ref order", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		idx, ok := firstIndex(entity)
		x.True(ok)
		x.Equal("child", idx.Name())
		x.True(idx.IsUnique())
		props := slices.Collect(idx.Props())
		x.Len(props, 2)
		x.Equal(protoreflect.FieldNumber(10), props[0].Number())
		x.Equal(protoreflect.FieldNumber(5), props[1].Number())
		// Number() reports the first prop's number.
		x.Equal(protoreflect.FieldNumber(10), idx.Number())
	}))
	t.Run("index must reference at least one prop", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_index_no_ref_proto)
		x.Error(err)
		x.ErrorContains(err, "index must reference at least one prop")
	}))
	t.Run("ref name mismatch reports a single, clear error", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_invalid_index_ref_name_mismatch_proto)
		x.Error(err)
		x.ErrorContains(err, "name not matched")
		// The reference IS found by number, so it must not also be reported as
		// "reference not found" (regression: duplicate/contradictory errors).
		x.Equal(1, strings.Count(err.Error(), "name not matched"))
		x.NotContains(err.Error(), "reference not found")
	}))
}

func firstIndex(entity graph.Entity) (graph.Index, bool) {
	for idx := range entity.Indexes() {
		return idx, true
	}
	return nil, false
}
