package graph_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/stretchr/testify/require"
)

func TestRpcEnable(t *testing.T) {
	t.Run("enabled with option", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcEnabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Rpcs().HasAdd())
		x.False(entity.Rpcs().HasGet())
		x.False(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasErase())
	}))
	t.Run("enabled by CRUD flag", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcCrud", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Rpcs().HasAdd())
		x.True(entity.Rpcs().HasGet())
		x.True(entity.Rpcs().HasPatch())
		x.True(entity.Rpcs().HasErase())
	}))
	t.Run("disable explicitly with CRUD flag", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcCrudExclude", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.Rpcs().HasAdd())
		x.True(entity.Rpcs().HasGet())
		x.True(entity.Rpcs().HasPatch())
		x.True(entity.Rpcs().HasErase())
	}))
	t.Run("disable since no option", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcDisabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.Rpcs().HasAdd())
		x.False(entity.Rpcs().HasGet())
		x.False(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasErase())
	}))
	t.Run("disable with option", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcDisabledExplicit", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.Rpcs().HasAdd())
		x.False(entity.Rpcs().HasGet())
		x.False(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasErase())
	}))
}
