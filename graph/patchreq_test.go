package graph_test

import (
	"context"
	"testing"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestPatchSlots(t *testing.T) {
	t.Run("a prop's value and its companion", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, e graph.Entity) {
		props := map[string]graph.Prop{}
		for p := range graph.PatchProps(e) {
			props[p.Name()] = p
		}

		// alias is nullable, so it carries `_null`; labels is a map, which has
		// no companion because there is nothing to distinguish -- an absent
		// map and an empty one are the same request.
		alias := props["alias"]
		x.NotNil(alias)
		x.EqualValues(8, graph.PatchValueNumber(alias))
		x.EqualValues(9, graph.PatchFlagNumber(alias))
		x.Equal(graph.PatchFlagNull, graph.PatchFlagOf(alias))
		x.Equal("alias_null", graph.PatchFlagName(alias))

		labels := props["labels"]
		x.NotNil(labels)
		x.EqualValues(14, graph.PatchValueNumber(labels))
		x.Equal(graph.PatchFlagNone, graph.PatchFlagOf(labels))
		x.Empty(graph.PatchFlagName(labels))
	}))

	t.Run("the version field carries force, not null", WithEntity(graphtest.File_graphtest_version_proto, "VersionField", func(x *require.Assertions, g *graph.Graph, e graph.Entity) {
		v := e.GetVersionField()
		x.NotNil(v)
		x.Equal(graph.PatchFlagForce, graph.PatchFlagOf(v))
		x.Equal("updated_at_force", graph.PatchFlagName(v))
		x.EqualValues(4, graph.PatchValueNumber(v))
		x.EqualValues(5, graph.PatchFlagNumber(v))
	}))

	// An immutable prop has no request field at all: there is no shape for
	// something that cannot be written.
	t.Run("immutable props are omitted", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, e graph.Entity) {
		var all, patchable int
		for p := range e.Props() {
			all++
			if p.IsImmutable() {
				continue
			}
			patchable++
		}
		x.Greater(all, patchable, "the fixture must have at least one immutable prop")

		var got int
		for p := range graph.PatchProps(e) {
			x.False(p.IsImmutable(), "%q is immutable and must not be in the request", p.Name())
			got++
		}
		x.Equal(patchable, got)
	}))

	// ref does not depend on the key. Prop numbers start at 1 and the lowest
	// slot a prop can claim is 2, so slot 1 is free whatever the schema says.
	t.Run("ref is 1 and no prop can reach it", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, e graph.Entity) {
		x.EqualValues(1, graph.PatchRefNumber(e))
		for p := range graph.PatchProps(e) {
			x.Greater(graph.PatchValueNumber(p), protoreflect.FieldNumber(1), "%q reaches ref", p.Name())
		}
	}))
}

func TestPatchLayout(t *testing.T) {
	t.Run("every slot resolves back to what put it there", WithEntity(library.File_library_user_proto, "User", func(x *require.Assertions, g *graph.Graph, e graph.Entity) {
		got, err := graph.PatchLayout(e)
		x.NoError(err)

		ref, ok := got[graph.PatchRefNumber(e)]
		x.True(ok)
		x.Equal(graph.PatchSlotRef, ref.Kind)
		x.Nil(ref.Prop)

		var want int
		for p := range graph.PatchProps(e) {
			want++
			v, ok := got[graph.PatchValueNumber(p)]
			x.True(ok, "no slot for %q", p.Name())
			x.Equal(graph.PatchSlotValue, v.Kind)
			x.Equal(p.Name(), v.Prop.Name())

			if graph.PatchFlagOf(p) == graph.PatchFlagNone {
				continue
			}
			want++
			f, ok := got[graph.PatchFlagNumber(p)]
			x.True(ok, "no companion slot for %q", p.Name())
			x.Equal(graph.PatchSlotFlag, f.Kind)
			x.Equal(p.Name(), f.Prop.Name())
		}
		x.Len(got, want+1, "ref plus every value and companion")
	}))

	// The key's number is no longer part of the layout. Under the old
	// arithmetic a key at 3 took the slot of the prop numbered 2; now ref is
	// pinned below every slot and the key can sit anywhere. This is what makes
	// a composite key a question about the Ref message rather than about
	// numbering.
	t.Run("a key that is not number 1 is fine", func(t *testing.T) {
		x := require.New(t)

		e := hazard(x, "KeyAtThree", []*descriptorpb.FieldDescriptorProto{
			field("name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
			field("id", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				ormpb.FieldOptions_builder{Key: proto.Bool(true)}.Build()),
		})
		x.EqualValues(3, e.Key().Number())

		got, err := graph.PatchLayout(e)
		x.NoError(err)
		x.EqualValues(1, graph.PatchRefNumber(e))
		x.Equal(graph.PatchSlotRef, got[1].Kind)
		x.Equal("name", got[4].Prop.Name())
	})

	t.Run("the same props with the key at 1 land in the same slots", func(t *testing.T) {
		x := require.New(t)

		e := hazard(x, "KeyAtOne", []*descriptorpb.FieldDescriptorProto{
			field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				ormpb.FieldOptions_builder{Key: proto.Bool(true)}.Build()),
			field("name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
		})

		got, err := graph.PatchLayout(e)
		x.NoError(err)
		x.Equal(graph.PatchSlotRef, got[1].Kind)
		x.Equal("name", got[4].Prop.Name())
	})

	// Doubling a prop number can walk into the band protobuf keeps for itself,
	// where the field would be rejected by protoc rather than by us.
	t.Run("a prop whose slot lands in the reserved band is refused", func(t *testing.T) {
		x := require.New(t)

		e := hazard(x, "Reserved", []*descriptorpb.FieldDescriptorProto{
			field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				ormpb.FieldOptions_builder{Key: proto.Bool(true)}.Build()),
			// 9501*2 = 19002, inside 19000-19999.
			field("name", 9501, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
		})

		_, err := graph.PatchLayout(e)
		x.ErrorContains(err, "reserved range")
		t.Log(err)
	})
}

// hazard parses a synthetic entity, for the layouts no fixture has on purpose.
func hazard(x *require.Assertions, name string, fields []*descriptorpb.FieldDescriptorProto) graph.Entity {
	mopts := &descriptorpb.MessageOptions{}
	proto.SetExtension(mopts, ormpb.E_Message, &ormpb.MessageOptions{})

	file := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("graphtest/hazard_" + name + ".proto"),
		Package:    proto.String("graphtest.hazard"),
		Syntax:     proto.String("editions"),
		Edition:    descriptorpb.Edition_EDITION_2023.Enum(),
		Dependency: []string{string(ormpb.E_Field.TypeDescriptor().ParentFile().Path())},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name:    proto.String(name),
			Field:   fields,
			Options: mopts,
		}},
	}

	fd, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
	x.NoError(err)

	g := graph.NewGraph()
	x.NoError(graph.Parse(context.Background(), g, fd))

	e, ok := g.Entities[fd.Messages().Get(0).FullName()]
	x.True(ok, "entities: %v", g.Entities)
	return e
}

func field(
	name string,
	number int32,
	kind descriptorpb.FieldDescriptorProto_Type,
	opts *ormpb.FieldOptions,
) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   kind.Enum(),
	}
	if opts != nil {
		f.Options = &descriptorpb.FieldOptions{}
		proto.SetExtension(f.Options, ormpb.E_Field, opts)
	}
	return f
}
