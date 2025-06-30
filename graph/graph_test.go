package graph_test

import (
	"slices"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestSingleMessage(t *testing.T) {
	x := require.New(t)

	g := graph.NewGraph()
	err := graph.Parse(t.Context(), g, library.File_library_user_proto)
	x.NoError(err)
	x.Len(g.Entities, 1)

	msg_name := library.File_library_user_proto.FullName().Append("User")
	x.Contains(g.Entities, msg_name)

	entity := g.Entities[msg_name]
	x.NotNil(entity)
	x.Equal(msg_name, entity.FullName())

	props := slices.Collect(entity.Props())
	x.Len(props, 7)

	i := 0
	take := func() graph.Prop {
		v := props[i]
		i++
		return v
	}

	{
		p := take()
		x.Implements((*graph.Field)(nil), p)
		v := p.(graph.Field)
		x.Equal(msg_name.Append("id"), v.FullName())
		x.Equal(protoreflect.FieldNumber(1), v.Number())
		x.Equal(ormpb.Type_TYPE_UUID, v.Type())
	}
	{
		p := take()
		x.Implements((*graph.Field)(nil), p)
		v := p.(graph.Field)
		x.Equal(msg_name.Append("name"), v.FullName())
		x.Equal(protoreflect.FieldNumber(2), v.Number())
		x.Equal(ormpb.Type_TYPE_STRING, v.Type())
	}
	{
		p := take()
		x.Implements((*graph.Field)(nil), p)
		v := p.(graph.Field)
		x.Equal(msg_name.Append("labels"), v.FullName())
		x.Equal(protoreflect.FieldNumber(5), v.Number())
		x.Equal(ormpb.Type_TYPE_JSON, v.Type())
	}
	{
		p := take()
		x.Implements((*graph.Field)(nil), p)
		v := p.(graph.Field)
		x.Equal(msg_name.Append("age"), v.FullName())
		x.Equal(protoreflect.FieldNumber(4), v.Number())
		x.Equal(ormpb.Type_TYPE_UINT32, v.Type())
	}
	{
		p := take()
		x.Implements((*graph.Edge)(nil), p)
		v := p.(graph.Edge)
		x.Equal(msg_name.Append("parent"), v.FullName())
		x.Equal(protoreflect.FieldNumber(6), v.Number())
		x.Same(entity, v.Target())
		x.Nil(v.Reverse())
		x.NotNil(v.Inverse())
		x.Equal(msg_name.Append("children"), v.Inverse().FullName())
	}
	{
		p := take()
		x.Implements((*graph.Edge)(nil), p)
		v := p.(graph.Edge)
		x.Equal(msg_name.Append("children"), v.FullName())
		x.Equal(protoreflect.FieldNumber(7), v.Number())
		x.Same(entity, v.Target())
		x.NotNil(v.Reverse())
		x.Nil(v.Inverse())
		x.Equal(msg_name.Append("parent"), v.Reverse().FullName())
	}
	{
		p := take()
		x.Implements((*graph.Field)(nil), p)
		v := p.(graph.Field)
		x.Equal(msg_name.Append("date_created"), v.FullName())
		x.Equal(protoreflect.FieldNumber(15), v.Number())
		x.Equal(ormpb.Type_TYPE_TIME, v.Type())
	}
}
