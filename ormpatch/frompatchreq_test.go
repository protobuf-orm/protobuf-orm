// What FromPatchRequest does with each shape a PatchRequest can take.
//
// Every case runs the whole way through -- convert, validate, compile -- so a
// document that is merely well-formed but does not compile fails here rather
// than in a backend.

package ormpatch_test

import (
	"fmt"
	"testing"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/ormpatch"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestRequestLayout(t *testing.T) {
	t.Run("library.User", withUser(func(p *probe) {
		t.Log("UserPatchRequest:" + p.layout())

		want := map[string]protoreflect.FieldNumber{
			"ref":        1, // pinned, independent of the key
			"alias":      8, // N=4
			"alias_null": 9,
			"name":       10, // N=5
			"name_null":  11,
			"desc":       12, // N=6
			"desc_null":  13,
			"labels":     14, // N=7, map: no companion
			"parent":     20, // N=10, edge: not nullable, no companion
			"children":   22, // N=11, repeated edge
		}
		for name, num := range want {
			fd := p.fd(name)
			p.x.Equal(num, fd.Number(), "field %q", name)
		}
		p.x.Equal(len(want), p.md.Fields().Len(), "unexpected extra fields:"+p.layout())

		// id (key) and date_created are immutable; metadata carries
		// orm.field.disabled so it is not a prop at all.
		for _, gone := range []string{"id", "date_created", "metadata"} {
			p.x.Nil(p.md.Fields().ByName(protoreflect.Name(gone)), "%q must not be in the request", gone)
		}
	}))

	t.Run("graphtest.VersionField", withVersioned(func(p *probe) {
		t.Log("VersionFieldPatchRequest:" + p.layout())

		p.x.EqualValues(1, p.fd("ref").Number())
		p.x.EqualValues(4, p.fd("updated_at").Number()) // N=2
		p.x.EqualValues(5, p.fd("updated_at_force").Number())
		p.x.EqualValues(6, p.fd("name").Number()) // N=3
		p.x.EqualValues(7, p.fd("name_null").Number())
	}))

	t.Run("no two slots collide", withUser(func(p *probe) {
		seen := map[protoreflect.FieldNumber]string{}
		for i := range p.md.Fields().Len() {
			fd := p.md.Fields().Get(i)
			prev, dup := seen[fd.Number()]
			p.x.False(dup, "%d is both %q and %q", fd.Number(), prev, fd.Name())
			seen[fd.Number()] = string(fd.Name())
			p.x.False(fd.Number() >= 19000 && fd.Number() <= 19999,
				"%q lands in protobuf's reserved range", fd.Name())
		}
	}))

	// `ref` is pinned at 1 and no prop can reach it, so the key's own number
	// stops mattering. Under the previous layout this entity could not have a
	// request at all.
	t.Run("a key that is not number 1 builds anyway", func(t *testing.T) {
		x := require.New(t)

		e := hazardEntity(x, "KeyAtThree", []*descriptorpb.FieldDescriptorProto{
			ormField("name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
			ormField("id", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				ormpb.FieldOptions_builder{Key: proto.Bool(true)}.Build()),
		})
		x.EqualValues(3, e.Key().Number())

		md, err := buildPatchRequest(e)
		x.NoError(err)
		x.EqualValues(1, md.Fields().ByName("ref").Number())
		x.EqualValues(4, md.Fields().ByName("name").Number())
	})

	t.Run("a key at 1 puts the props in the same slots", func(t *testing.T) {
		x := require.New(t)

		e := hazardEntity(x, "KeyAtOne", []*descriptorpb.FieldDescriptorProto{
			ormField("id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				ormpb.FieldOptions_builder{Key: proto.Bool(true)}.Build()),
			ormField("name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
		})
		x.EqualValues(1, e.Key().Number())

		md, err := buildPatchRequest(e)
		x.NoError(err)
		x.EqualValues(1, md.Fields().ByName("ref").Number())
		x.EqualValues(4, md.Fields().ByName("name").Number())
	})
}

// hazardEntity builds an entity at runtime so the numbering rule can be probed
// with a key that no committed fixture has.
func TestScalarAssign(t *testing.T) {
	t.Run("a set scalar becomes SetColumn", withUser(func(p *probe) {
		p.set("name", protoreflect.ValueOfString("Ada"))

		plan := p.run(nil)
		p.x.Empty(plan.Tests)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("name", plan.Writes[0].Prop.Name())

		op, ok := plan.Writes[0].Op.(ormpatch.SetColumn)
		p.x.True(ok, "op is %T", plan.Writes[0].Op)
		p.x.Equal("Ada", op.Value.Interface())
	}))

	t.Run("an unset scalar produces nothing", withUser(func(p *probe) {
		doc, err := p.convert(nil)
		p.x.NoError(err)
		p.x.Nil(doc, "an empty request must be absence, not an empty Delta")
	}))

	t.Run("the zero value is still a write when present", withUser(func(p *probe) {
		p.set("name", protoreflect.ValueOfString(""))

		plan := p.run(nil)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("", plan.Writes[0].Op.(ormpatch.SetColumn).Value.Interface())
	}))

	t.Run("several props in one document", withUser(func(p *probe) {
		p.set("name", protoreflect.ValueOfString("Ada"))
		p.set("alias", protoreflect.ValueOfString("ada"))

		plan := p.run(nil)
		p.x.Len(plan.Writes, 2)
		p.x.Equal("ada", writeTo(p.x, plan, "alias").Op.(ormpatch.SetColumn).Value.Interface())
		p.x.Equal("Ada", writeTo(p.x, plan, "name").Op.(ormpatch.SetColumn).Value.Interface())
	}))
}

func TestMapAssign(t *testing.T) {
	t.Run("a whole map becomes SetColumn, not EditJSON", withUser(func(p *probe) {
		p.setMap("labels", map[string]string{"env": "prod", "team": "core"})

		plan := p.run(nil)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("labels", plan.Writes[0].Prop.Name())

		op, ok := plan.Writes[0].Op.(ormpatch.SetColumn)
		p.x.True(ok, "op is %T", plan.Writes[0].Op)

		got := map[string]string{}
		op.Value.Map().Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
			got[k.String()] = v.String()
			return true
		})
		p.x.Equal(map[string]string{"env": "prod", "team": "core"}, got)
	}))

	t.Run("an empty map is indistinguishable from absent", withUser(func(p *probe) {
		// Preserves today's `if len(u) > 0` gate: Patch cannot clear a map.
		p.set("name", protoreflect.ValueOfString("Ada"))
		p.req.Mutable(p.fd("labels"))

		plan := p.run(nil)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("name", plan.Writes[0].Prop.Name())
	}))
}

func TestNullableClear(t *testing.T) {
	t.Run("_null becomes ClearColumn", withUser(func(p *probe) {
		p.set("desc_null", protoreflect.ValueOfBool(true))

		plan := p.run(nil)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("desc", plan.Writes[0].Prop.Name())
		p.x.IsType(ormpatch.ClearColumn{}, plan.Writes[0].Op)
	}))

	t.Run("_null wins over a value on the same prop", withUser(func(p *probe) {
		// Two entries on one column fold with the LAST winning, so the
		// converter must emit exactly one.
		p.set("desc", protoreflect.ValueOfString("ignored"))
		p.set("desc_null", protoreflect.ValueOfBool(true))

		plan := p.run(nil)
		p.x.Len(plan.Writes, 1)
		p.x.IsType(ormpatch.ClearColumn{}, plan.Writes[0].Op)
	}))

	t.Run("_null false leaves the value path alone", withUser(func(p *probe) {
		p.set("desc", protoreflect.ValueOfString("kept"))
		p.set("desc_null", protoreflect.ValueOfBool(false))

		plan := p.run(nil)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("kept", plan.Writes[0].Op.(ormpatch.SetColumn).Value.Interface())
	}))
}

func TestVersion(t *testing.T) {
	t.Run("the version becomes a test, not a write", withVersioned(func(p *probe) {
		p.setTime("updated_at", 1700000000, 123)
		p.set("name", protoreflect.ValueOfString("Ada"))

		plan := p.run(nil)
		p.x.Len(plan.Tests, 1)
		p.x.Equal("updated_at", plan.Tests[0].Prop.Name())
		p.x.Equal(ormpatch.TestEqual, plan.Tests[0].Want)

		ts := plan.Tests[0].Value.Message()
		p.x.EqualValues(1700000000, ts.Get(ts.Descriptor().Fields().ByName("seconds")).Int())
		p.x.EqualValues(123, ts.Get(ts.Descriptor().Fields().ByName("nanos")).Int())

		// The version is not written; only the other prop is.
		p.x.Len(plan.Writes, 1)
		p.x.Equal("name", plan.Writes[0].Prop.Name())
		_, wrote := findWrite(plan, "updated_at")
		p.x.False(wrote, "the version column must be left for the server to stamp")
	}))

	t.Run("no version and no force is refused", withVersioned(func(p *probe) {
		p.set("name", protoreflect.ValueOfString("Ada"))

		_, err := p.convert(nil)
		p.x.ErrorContains(err, "version not given: updated_at")
	}))

	t.Run("force with a value assigns it and tests nothing", withVersioned(func(p *probe) {
		p.set("updated_at_force", protoreflect.ValueOfBool(true))
		p.setTime("updated_at", 999, 0)

		plan := p.run(nil)
		p.x.Empty(plan.Tests, "force means no precondition")
		p.x.Len(plan.Writes, 1)
		p.x.Equal("updated_at", plan.Writes[0].Prop.Name())

		op, ok := plan.Writes[0].Op.(ormpatch.SetColumn)
		p.x.True(ok, "op is %T", plan.Writes[0].Op)
		ts := op.Value.Message()
		p.x.EqualValues(999, ts.Get(ts.Descriptor().Fields().ByName("seconds")).Int())
		_, wrote := findWrite(plan, "updated_at")
		p.x.True(wrote)
	}))

	t.Run("force without a value neither tests nor writes the version", withVersioned(func(p *probe) {
		p.set("updated_at_force", protoreflect.ValueOfBool(true))
		p.set("name", protoreflect.ValueOfString("Ada"))

		plan := p.run(nil)
		p.x.Empty(plan.Tests)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("name", plan.Writes[0].Prop.Name())
	}))

	t.Run("a version-only request is a legal test-only plan", withVersioned(func(p *probe) {
		p.setTime("updated_at", 42, 0)

		plan := p.run(nil)
		p.x.Empty(plan.Writes)
		p.x.Len(plan.Tests, 1)
		p.x.False(plan.IsEmpty())
	}))

	// Entries come out in declaration order, so an entity whose version field
	// is declared LAST emits its test after writes to other columns. Compile
	// refuses a test after a write to the SAME column; a different column is
	// fine, and that is what this checks -- the converter does not have to sort.
	t.Run("a version declared last still compiles", func(t *testing.T) {
		x := require.New(t)

		e := hazardEntity(x, "VersionLast", []*descriptorpb.FieldDescriptorProto{
			ormField("id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				ormpb.FieldOptions_builder{Key: proto.Bool(true)}.Build()),
			ormField("name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
			ormTimestamp("updated_at", 3,
				ormpb.FieldOptions_builder{Version: &ormpb.VersionOptions{}}.Build()),
		})
		x.True(e.HasVersionField())

		md, err := buildPatchRequest(e)
		x.NoError(err)

		p := &probe{x: x, e: e, md: md, req: dynamicpb.NewMessage(md)}
		p.set("name", protoreflect.ValueOfString("Ada"))
		p.setTime("updated_at", 7, 0)

		doc, err := p.convert(nil)
		x.NoError(err)
		// Assign(name) comes before Test(updated_at) in the document.
		x.Equal(patchpb.Entry_Assign_case, doc.GetDelta().GetEntries()[0].WhichKind())
		x.Equal(patchpb.Entry_Test_case, doc.GetDelta().GetEntries()[1].WhichKind())

		plan := p.run(nil)
		x.Len(plan.Writes, 1)
		x.Equal("name", plan.Writes[0].Prop.Name())
		x.Len(plan.Tests, 1)
		x.Equal("updated_at", plan.Tests[0].Prop.Name())
	})
}

func TestEdge(t *testing.T) {
	// resolveByID reads the Ref's `id` arm. A real resolver would fall back to
	// a database read for a ref that names a non-key field.
	resolveByID := func(ed graph.Edge, ref protoreflect.Message) (protoreflect.Value, error) {
		fd := ref.Descriptor().Fields().ByName(ed.Target().Key().Descriptor().Name())
		if fd == nil || !ref.Has(fd) {
			return protoreflect.Value{}, fmt.Errorf("ref carries no key")
		}
		return ref.Get(fd), nil
	}

	t.Run("an edge repoint becomes SetEdge", withUser(func(p *probe) {
		k := []byte("0123456789abcdef")
		p.setRef("parent", "id", protoreflect.ValueOfBytes(k))

		plan := p.run(resolveByID)
		p.x.Len(plan.Writes, 1)
		p.x.Equal("parent", plan.Writes[0].Prop.Name())

		op, ok := plan.Writes[0].Op.(ormpatch.SetEdge)
		p.x.True(ok, "op is %T", plan.Writes[0].Op)
		p.x.Equal(k, op.Key.Interface())
	}))

	t.Run("an edge alongside a scalar", withUser(func(p *probe) {
		k := []byte("0123456789abcdef")
		p.setRef("parent", "id", protoreflect.ValueOfBytes(k))
		p.set("name", protoreflect.ValueOfString("Ada"))

		plan := p.run(resolveByID)
		p.x.Len(plan.Writes, 2)
		p.x.IsType(ormpatch.SetEdge{}, writeTo(p.x, plan, "parent").Op)
		p.x.IsType(ormpatch.SetColumn{}, writeTo(p.x, plan, "name").Op)
	}))

	t.Run("an unset edge does not call the resolver", withUser(func(p *probe) {
		p.set("name", protoreflect.ValueOfString("Ada"))

		plan := p.run(func(graph.Edge, protoreflect.Message) (protoreflect.Value, error) {
			p.x.FailNow("resolver must not be called for an absent edge")
			return protoreflect.Value{}, nil
		})
		p.x.Len(plan.Writes, 1)
	}))

	t.Run("a resolver error is surfaced, not swallowed", withUser(func(p *probe) {
		p.setRef("parent", "id", protoreflect.ValueOfBytes([]byte("0123456789abcdef")))

		_, err := p.convert(func(graph.Edge, protoreflect.Message) (protoreflect.Value, error) {
			return protoreflect.Value{}, fmt.Errorf("no such tenant")
		})
		p.x.ErrorContains(err, "no such tenant")
	}))

	t.Run("an empty key is refused, not written", withUser(func(p *probe) {
		// A resolver that finds nothing has no key to give. Whatever it returns
		// must not reach the plan: an edge column holds a UUID, and neither a
		// nil slice nor an empty one is one.
		//
		// Both reach the same refusal, which is the point: nil and empty are
		// one thing here, and neither is a key. They used to diverge -- a nil
		// slice left the arm unset and was refused by position inside the
		// document rather than by name -- which is what the converter's own
		// normalization still guards against on an older protobuf-patch.
		p.setRef("parent", "id", protoreflect.ValueOfBytes([]byte("0123456789abcdef")))

		for name, key := range map[string]protoreflect.Value{
			"nil":   protoreflect.ValueOfBytes(nil),
			"empty": protoreflect.ValueOfBytes([]byte{}),
		} {
			doc, err := p.convert(func(graph.Edge, protoreflect.Message) (protoreflect.Value, error) {
				return key, nil
			})
			p.x.NoError(err, "%s: converting must not fail on the arm", name)
			p.x.NoError(patch.Validate(doc), "%s: the document must be well-formed", name)

			_, err = ormpatch.Compile(p.e, doc)
			var bad *ormpatch.InvalidValueError
			p.x.ErrorAs(err, &bad, "%s", name)
			p.x.Contains(err.Error(), "16 bytes, got 0", "%s", name)
		}
	}))

	t.Run("a repeated edge is refused rather than silently dropped", withUser(func(p *probe) {
		fd := p.fd("children")
		l := p.req.Mutable(fd).List()
		r := l.NewElement().Message()
		r.Set(r.Descriptor().Fields().ByName("id"), protoreflect.ValueOfBytes([]byte("0123456789abcdef")))
		l.Append(protoreflect.ValueOfMessage(r))

		_, err := p.convert(resolveByID)
		p.x.ErrorContains(err, "repeated edge")
	}))
}

func TestEverythingAtOnce(t *testing.T) {
	t.Run("one request, one plan", withUser(func(p *probe) {
		k := []byte("0123456789abcdef")
		p.set("name", protoreflect.ValueOfString("Ada"))
		p.set("desc_null", protoreflect.ValueOfBool(true))
		p.setMap("labels", map[string]string{"env": "prod"})
		p.setRef("parent", "id", protoreflect.ValueOfBytes(k))

		plan := p.run(func(ed graph.Edge, ref protoreflect.Message) (protoreflect.Value, error) {
			return ref.Get(ref.Descriptor().Fields().ByName("id")), nil
		})

		p.x.Empty(plan.Tests)
		p.x.Len(plan.Writes, 4)
		p.x.IsType(ormpatch.SetColumn{}, writeTo(p.x, plan, "name").Op)
		p.x.IsType(ormpatch.ClearColumn{}, writeTo(p.x, plan, "desc").Op)
		p.x.IsType(ormpatch.SetColumn{}, writeTo(p.x, plan, "labels").Op)
		p.x.IsType(ormpatch.SetEdge{}, writeTo(p.x, plan, "parent").Op)
	}))
}
