package graph_test

import (
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func WithEntity(d protoreflect.FileDescriptor, name string, f func(x *require.Assertions, g *graph.Graph, entity graph.Entity)) func(t *testing.T) {
	return func(t *testing.T) {
		x := require.New(t)

		g := graph.NewGraph()
		err := graph.Parse(t.Context(), g, d)
		x.NoError(err)

		m := d.Messages().ByName(protoreflect.Name(name))
		x.NotNil(m)

		entity, ok := g.Entities[m.FullName()]
		x.True(ok)

		f(x, g, entity)
	}
}
