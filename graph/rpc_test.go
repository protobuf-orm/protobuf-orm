package graph_test

import (
	"context"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestRpcEnable(t *testing.T) {
	t.Run("enabled with option", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcEnabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Rpcs().HasAdd())
		x.False(entity.Rpcs().HasGet())
		x.False(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasErase())
	}))
	t.Run("the five named and nothing else", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcAll", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Rpcs().HasAdd())
		x.True(entity.Rpcs().HasGet())
		x.True(entity.Rpcs().HasPatch())
		x.True(entity.Rpcs().HasApply())
		x.True(entity.Rpcs().HasErase())
	}))
	t.Run("disable explicitly", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcAddDisabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.Rpcs().HasAdd())
		x.True(entity.Rpcs().HasGet())
		x.True(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasApply())
		x.True(entity.Rpcs().HasErase())
	}))
	t.Run("none from an rpc that names none", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcEmpty", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.Rpcs().HasAdd())
		x.False(entity.Rpcs().HasAddMany())
		x.False(entity.Rpcs().HasGet())
		x.False(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasApply())
		x.False(entity.Rpcs().HasErase())
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

func TestRpcAddMany(t *testing.T) {
	t.Run("enabled beside Add", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcAddMany", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		rpcs := entity.Rpcs()
		x.True(rpcs.HasAdd())
		x.True(rpcs.HasAddMany())

		v := rpcs.GetAddMany()
		x.Equal("graphtest.RpcAddManyService.AddMany", string(v.FullName()))
		x.Equal("graphtest.RpcAddManyAddManyRequest", string(v.Request().FullName()))
		x.Equal("graphtest.RpcAddManyAddManyResponse", string(v.Response().FullName()))
		x.False(v.Request().IsStream())
		x.False(v.Response().IsStream())
		x.Equal(graph.DefaultAddManyMax, v.Max())
	}))
	t.Run("with a max of its own", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcAddManyMax", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.Equal(50, entity.Rpcs().GetAddMany().Max())
	}))
	t.Run("not without its Add", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcAddManyWithoutAdd", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.Rpcs().HasAdd())
		x.False(entity.Rpcs().HasAddMany())
		x.Nil(entity.Rpcs().GetAddMany())
	}))
	t.Run("and not by naming every other verb", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcAll", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Rpcs().HasAdd())
		x.False(entity.Rpcs().HasAddMany())
		x.Nil(entity.Rpcs().GetAddMany())
	}))
}

// The names are the ones protoc-gen-orm-service writes, request and answer
// for every verb.
func TestRpcMessageNames(t *testing.T) {
	WithEntity(graphtest.File_graphtest_rpc_proto, "RpcAll", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		rpcs := entity.Rpcs()

		add := rpcs.GetAdd()
		x.Equal("graphtest.RpcAllService.Add", string(add.FullName()))
		x.Equal("graphtest.RpcAllAddRequest", string(add.Request().FullName()))
		x.Equal("graphtest.RpcAll", string(add.Response().FullName()))
		x.False(add.Request().IsStream())
		x.False(add.Response().IsStream())

		get := rpcs.GetGet()
		x.Equal("graphtest.RpcAllService.Get", string(get.FullName()))
		x.Equal("graphtest.RpcAllGetRequest", string(get.Request().FullName()))
		x.Equal("graphtest.RpcAll", string(get.Response().FullName()))

		patch := rpcs.GetPatch()
		x.Equal("graphtest.RpcAllService.Patch", string(patch.FullName()))
		x.Equal("graphtest.RpcAllPatchRequest", string(patch.Request().FullName()))
		x.Equal("graphtest.RpcAll", string(patch.Response().FullName()))

		apply := rpcs.GetApply()
		x.Equal("graphtest.RpcAllService.Apply", string(apply.FullName()))
		x.Equal("graphtest.RpcAllApplyRequest", string(apply.Request().FullName()))
		x.Equal("graphtest.RpcAll", string(apply.Response().FullName()))
		x.False(apply.Request().IsStream())
		x.False(apply.Response().IsStream())

		erase := rpcs.GetErase()
		x.Equal("graphtest.RpcAllService.Erase", string(erase.FullName()))
		x.Equal("graphtest.RpcAllRef", string(erase.Request().FullName()))
		x.Equal("graphtest.RpcAllEraseResponse", string(erase.Response().FullName()))

		x.Same(entity, add.Entity())
	})(t)
}

func TestRpcApply(t *testing.T) {
	t.Run("not brought by Patch", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcPatchAlone", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasApply())
		x.Nil(entity.Rpcs().GetApply())
	}))
	t.Run("without Patch", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcApplyAlone", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.False(entity.Rpcs().HasPatch())
		x.True(entity.Rpcs().HasApply())
		x.Equal("graphtest.RpcApplyAloneService.Apply", string(entity.Rpcs().GetApply().FullName()))
	}))
	t.Run("disabled", WithEntity(graphtest.File_graphtest_rpc_proto, "RpcApplyDisabled", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Rpcs().HasPatch())
		x.False(entity.Rpcs().HasApply())
		x.Nil(entity.Rpcs().GetApply())
	}))
}

// fileWithRpc is a file of one entity whose `orm.message.rpc` is rpc, as a
// schema compiled against some `orm` would hand it over.
func fileWithRpc(x *require.Assertions, rpc *ormpb.RpcOptions) protoreflect.FileDescriptor {
	mo := &descriptorpb.MessageOptions{}
	proto.SetExtension(mo, ormpb.E_Message, ormpb.MessageOptions_builder{Rpc: rpc}.Build())

	fo := &descriptorpb.FieldOptions{}
	proto.SetExtension(fo, ormpb.E_Field, ormpb.FieldOptions_builder{Key: proto.Bool(true)}.Build())

	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    proto.String("rpctest/unknown.proto"),
		Package: proto.String("rpctest"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Thing"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:     proto.String("id"),
				Number:   proto.Int32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
				JsonName: proto.String("id"),
				Options:  fo,
			}},
			Options: mo,
		}},
	}, protoregistry.GlobalFiles)
	x.NoError(err)

	return fd
}

// withUnknown is rpc carrying field n as bytes this `orm` has no name for --
// what a schema compiled against another `orm` arrives as.
func withUnknown(rpc *ormpb.RpcOptions, n protowire.Number) *ormpb.RpcOptions {
	b := protowire.AppendTag(nil, n, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	rpc.ProtoReflect().SetUnknown(b)
	return rpc
}

func TestRpcOptionsUnknown(t *testing.T) {
	t.Run("crud is refused by name", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		rpc := withUnknown(ormpb.RpcOptions_builder{Get: &ormpb.RpcGet{}}.Build(), 2)
		err := graph.Parse(ctx, g, fileWithRpc(x, rpc))
		x.Error(err)
		x.ErrorContains(err, "rpctest.Thing: rpc: crud is gone; name each verb the entity has (add, get, patch, apply, erase), and Apply no longer comes with Patch")
		x.Empty(g.Entities)
	}))
	t.Run("another number is another orm", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		rpc := withUnknown(ormpb.RpcOptions_builder{Get: &ormpb.RpcGet{}}.Build(), 99)
		err := graph.Parse(ctx, g, fileWithRpc(x, rpc))
		x.Error(err)
		x.ErrorContains(err, "rpctest.Thing: rpc: field 99 is unknown; the schema was compiled against another orm than this generator was built with")
		x.Empty(g.Entities)
	}))
	t.Run("and none is not refused", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		rpc := ormpb.RpcOptions_builder{Get: &ormpb.RpcGet{}}.Build()
		x.NoError(graph.Parse(ctx, g, fileWithRpc(x, rpc)))
		x.True(g.Entities["rpctest.Thing"].Rpcs().HasGet())
	}))
}

// The options a schema hands in are the schema's: parsing reads them and
// leaves them as they were.
func TestParseDoesNotMutateRpcOptions(t *testing.T) {
	x := require.New(t)
	f := graphtest.File_graphtest_rpc_proto
	read := func(md protoreflect.MessageDescriptor) *ormpb.MessageOptions {
		return proto.GetExtension(md.Options(), ormpb.E_Message).(*ormpb.MessageOptions)
	}

	before := map[protoreflect.FullName]proto.Message{}
	for i := 0; i < f.Messages().Len(); i++ {
		md := f.Messages().Get(i)
		before[md.FullName()] = proto.Clone(read(md))
	}

	g := graph.NewGraph()
	x.NoError(graph.Parse(t.Context(), g, f))

	for i := 0; i < f.Messages().Len(); i++ {
		md := f.Messages().Get(i)
		x.True(proto.Equal(before[md.FullName()], read(md)), "%s: Parse must leave the options it is handed as they were", md.FullName())
	}
}
