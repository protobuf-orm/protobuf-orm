package graph_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestEntityEnable(t *testing.T) {
	t.Run("enabled with option", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_entity_proto)
		x.NoError(err)
		x.Contains(g.Entities, graphtest.File_graphtest_entity_proto.FullName().Append("EntityEnabled"))
	}))
	t.Run("disabled since no option", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_entity_proto)
		x.NoError(err)
		x.NotContains(g.Entities, graphtest.File_graphtest_entity_proto.FullName().Append("EntityDisabled"))
	}))
	t.Run("disabled with option", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_entity_proto)
		x.NoError(err)
		x.NotContains(g.Entities, graphtest.File_graphtest_entity_proto.FullName().Append("EntityDisabledExplicit"))
	}))
}

func TestEntityValidity(t *testing.T) {
	t.Run("key is unique", WithEntity(graphtest.File_graphtest_key_wo_unique_proto, "KeyWoUnique", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		x.True(entity.Key().IsUnique())
	}))
	t.Run("key cannot be set as no unique", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_key_no_unique_proto)
		x.Error(err)
		x.ErrorContains(err, "key must be unique")
	}))
	t.Run("key is not defined", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_key_no_proto)
		x.Error(err)
		x.ErrorContains(err, "no key is defined")
	}))
	t.Run("two or more keys are defined", WithGraph(func(ctx context.Context, x *require.Assertions, g *graph.Graph) {
		err := graph.Parse(ctx, g, graphtest.File_graphtest_key_many_proto)
		x.Error(err)
		x.ErrorContains(err, "there can be only one key")
		x.ErrorContains(err, "id:1")
		x.ErrorContains(err, "alias:2")
	}))
}

func TestEntityProps(t *testing.T) {
	withUserEntity := func(f func(x *require.Assertions, g *graph.Graph, entity graph.Entity)) func(t *testing.T) {
		return WithEntity(library.File_library_user_proto, "User", f)
	}
	withUserProp := func(name string, f func(x *require.Assertions, g *graph.Graph, entity graph.Entity, prop graph.Prop)) func(t *testing.T) {
		return withUserEntity(func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
			for prop := range entity.Props() {
				if string(prop.FullName().Name()) != name {
					continue
				}

				f(x, g, entity, prop)
				return
			}
		})
	}
	withUserField := func(name string, f func(x *require.Assertions, g *graph.Graph, entity graph.Entity, v graph.Field)) func(t *testing.T) {
		return withUserProp(name, func(x *require.Assertions, g *graph.Graph, entity graph.Entity, prop graph.Prop) {
			x.Implements((*graph.Field)(nil), prop)
			v := prop.(graph.Field)
			f(x, g, entity, v)
		})
	}
	withUserEdge := func(name string, f func(x *require.Assertions, g *graph.Graph, entity graph.Entity, v graph.Edge)) func(t *testing.T) {
		return withUserProp(name, func(x *require.Assertions, g *graph.Graph, entity graph.Entity, prop graph.Prop) {
			x.Implements((*graph.Edge)(nil), prop)
			v := prop.(graph.Edge)
			f(x, g, entity, v)
		})
	}

	t.Run("explicitly typed", withUserField("id", func(x *require.Assertions, g *graph.Graph, entity graph.Entity, field graph.Field) {
		x.Equal(ormpb.Type_TYPE_UUID, field.Type())
	}))

	// Implicitly typed.
	for _, tc := range []struct {
		name string
		from string
		to   ormpb.Type
	}{
		{
			name: "id",
			from: "string",
			to:   ormpb.Type_TYPE_STRING,
		},
		{
			name: "labels",
			from: "map<string,string>",
			to:   ormpb.Type_TYPE_JSON,
		},
		{
			name: "date_created",
			from: "google.protobuf.Timestamp",
			to:   ormpb.Type_TYPE_TIME,
		},
	} {
		t.Run(fmt.Sprintf("implicitly typed [%s]->[%s]", tc.from, tc.to.String()), withUserField(tc.from, func(x *require.Assertions, g *graph.Graph, entity graph.Entity, v graph.Field) {
			x.Equal(tc.to, v.Type())
		}))
	}

	t.Run("O2M same type parent", withUserEdge("parent", func(x *require.Assertions, g *graph.Graph, entity graph.Entity, v graph.Edge) {
		x.Same(entity, v.Target())
		x.NotNil(v.Reverse())
		x.Nil(v.Inverse())
		x.Equal(entity.FullName().Append("children"), v.Reverse().FullName())
	}))
	t.Run("O2M same type children", withUserEdge("children", func(x *require.Assertions, g *graph.Graph, entity graph.Entity, v graph.Edge) {
		x.Same(entity, v.Target())
		x.Nil(v.Reverse())
		x.NotNil(v.Inverse())
		x.Equal(entity.FullName().Append("parent"), v.Inverse().FullName())
	}))
	t.Run("numbers", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		vs := slices.Collect(entity.Props())
		x.Len(vs, 8)

		i := 0
		take := func() graph.Prop {
			v := vs[i]
			i++
			return v
		}
		for _, tc := range []struct {
			name   protoreflect.Name
			number protoreflect.FieldNumber
		}{
			{name: "id", number: 1},
			{name: "alias", number: 4},
			{name: "labels", number: 7},
			{name: "name", number: 5},
			{name: "desc", number: 6},
			// {name: "metadata", number: 8},
			{name: "parent", number: 10},
			{name: "children", number: 11},
			{name: "date_created", number: 15},
		} {
			p := take()
			x.Equal(p.FullName().Name(), tc.name)
			x.Equal(p.Number(), tc.number)
		}
	}))
}

func TestEntityIndexes(t *testing.T) {
	t.Run("valid", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, entity graph.Entity) {
		msg_name := entity.FullName()

		vs := slices.Collect(entity.Indexes())
		x.Len(vs, 1)

		i := 0
		take := func() graph.Index {
			v := vs[i]
			i++
			return v
		}

		{
			v := take()
			x.Equal("child", v.Name())

			props := slices.Collect(v.Props())
			x.Len(props, 2)
			{
				p := props[0]
				x.Implements((*graph.Edge)(nil), p)
				v := p.(graph.Edge)
				x.Equal(msg_name.Append("parent"), v.FullName())
				x.Equal(protoreflect.FieldNumber(10), v.Number())
			}
			{
				p := props[1]
				x.Implements((*graph.Field)(nil), p)
				v := p.(graph.Field)
				x.Equal(msg_name.Append("name"), v.FullName())
				x.Equal(protoreflect.FieldNumber(5), v.Number())
			}
		}
	}))
}
