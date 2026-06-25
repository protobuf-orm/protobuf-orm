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

func TestRpcMessageNames(t *testing.T) {
	WithEntity(graphtest.File_graphtest_rpc_proto, "RpcCrud", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		rpcs := entity.Rpcs()

		add := rpcs.GetAdd()
		x.Equal("graphtest.RpcCrudService.Add", string(add.FullName()))
		x.Equal("graphtest.RpcCrudAddRequest", string(add.Request().FullName()))
		x.Equal("graphtest.RpcCrud", string(add.Response().FullName()))
		x.False(add.Request().IsStream())
		x.False(add.Response().IsStream())

		get := rpcs.GetGet()
		x.Equal("graphtest.RpcCrudService.Get", string(get.FullName()))
		x.Equal("graphtest.RpcCrudRef", string(get.Request().FullName()))
		x.Equal("graphtest.RpcCrud", string(get.Response().FullName()))

		patch := rpcs.GetPatch()
		x.Equal("graphtest.RpcCrudService.Patch", string(patch.FullName()))
		x.Equal("graphtest.RpcCrudPatchRequest", string(patch.Request().FullName()))
		x.Equal("google.protobuf.Empty", string(patch.Response().FullName()))

		erase := rpcs.GetErase()
		x.Equal("graphtest.RpcCrudService.Erase", string(erase.FullName()))
		x.Equal("graphtest.RpcCrudRef", string(erase.Request().FullName()))
		x.Equal("google.protobuf.Empty", string(erase.Response().FullName()))

		x.Same(entity, add.Entity())
	})(t)
}
