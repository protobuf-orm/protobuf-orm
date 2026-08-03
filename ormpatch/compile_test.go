package ormpatch_test

import (
	"context"
	"testing"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpatch"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// WithEntity resolves library.User, which carries one of everything this engine
// has to tell apart: a UUID key, plain scalars, a map, an edge to itself, a
// repeated edge, an immutable timestamp, and a field the ORM does not map.
func WithEntity(f func(x *require.Assertions, e graph.Entity)) func(*testing.T) {
	return func(t *testing.T) {
		x := require.New(t)

		g := graph.NewGraph()
		err := graph.Parse(context.Background(), g, library.File_library_user_proto)
		x.NoError(err)

		e, ok := g.Entities[library.File_library_user_proto.FullName().Append("User")]
		x.True(ok, "entities: %v", g.Entities)

		f(x, e)
	}
}

func compile(x *require.Assertions, e graph.Entity, ops ...patch.Op) (*ormpatch.Plan, error) {
	x.NotEmpty(ops)
	p, err := patch.New(string(e.FullName()), ops[0], ops[1:]...)
	x.NoError(err)
	return ormpatch.Compile(e, p)
}

func TestCompileColumn(t *testing.T) {
	t.Run("assign a scalar", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("name")).Assign(patch.Str("Ada")))
		x.NoError(err)

		x.Empty(plan.Tests)
		x.Len(plan.Writes, 1)
		x.Equal("name", plan.Writes[0].Prop.Name())

		op, ok := plan.Writes[0].Op.(ormpatch.SetColumn)
		x.True(ok, "op is %T", plan.Writes[0].Op)
		x.Equal("Ada", op.Value.Interface())
	}))

	t.Run("remove a scalar clears the column", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("desc")).Remove())
		x.NoError(err)

		x.Len(plan.Writes, 1)
		x.IsType(ormpatch.ClearColumn{}, plan.Writes[0].Op)
	}))

	t.Run("test becomes a predicate, not a write", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("name")).Test(patch.Str("Ada")))
		x.NoError(err)

		x.Empty(plan.Writes)
		x.Len(plan.Tests, 1)
		x.Equal(ormpatch.TestEqual, plan.Tests[0].Want)
		x.Equal("Ada", plan.Tests[0].Value.Interface())
	}))

	t.Run("writes to one column fold, last wins", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.Name("name")).Assign(patch.Str("first")),
			patch.Target(patch.Name("name")).Assign(patch.Str("second")),
		)
		x.NoError(err)

		x.Len(plan.Writes, 1)
		x.Equal("second", plan.Writes[0].Op.(ormpatch.SetColumn).Value.Interface())
	}))

	t.Run("the key is immutable", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("id")).Assign(patch.Bytes([]byte("k"))))
		x.Error(err)

		var v *ormpatch.ImmutableError
		x.ErrorAs(err, &v)
	}))

	t.Run("an immutable prop is refused", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("date_created")).Remove())
		x.Error(err)

		var v *ormpatch.ImmutableError
		x.ErrorAs(err, &v)
	}))

	t.Run("a field the ORM does not map is refused, not skipped", WithEntity(func(x *require.Assertions, e graph.Entity) {
		// metadata is declared on the message but carries orm.field.disabled,
		// so the row has no column for it. Silently dropping the write would be
		// doing less than the document asks.
		_, err := compile(x, e, patch.Target(patch.Name("metadata")).Assign(patch.Str("x")))
		x.ErrorIs(err, ormpatch.ErrUnsupported)
		x.Equal(patch.CodeOK, patch.CodeOf(err), "an engine limit must not look like a format violation")
	}))

	t.Run("a field the message does not declare is a format error", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("nope")).Assign(patch.Str("x")))
		x.Equal(patch.CodeVacantTarget, patch.CodeOf(err))
	}))
}

func TestCompileEdge(t *testing.T) {
	t.Run("assign through the edge writes the foreign key", WithEntity(func(x *require.Assertions, e graph.Entity) {
		k := []byte("0123456789abcdef")
		plan, err := compile(x, e,
			patch.Target(patch.Name("id")).In(patch.Name("parent")).Assign(patch.Bytes(k)),
		)
		x.NoError(err)

		x.Len(plan.Writes, 1)
		x.Equal("parent", plan.Writes[0].Prop.Name())

		op, ok := plan.Writes[0].Op.(ormpatch.SetEdge)
		x.True(ok, "op is %T", plan.Writes[0].Op)
		x.Equal(k, op.Key.Interface())
	}))

	t.Run("test through the edge is a scalar comparison", WithEntity(func(x *require.Assertions, e graph.Entity) {
		k := []byte("0123456789abcdef")
		plan, err := compile(x, e,
			patch.Target(patch.Name("id")).In(patch.Name("parent")).Test(patch.Bytes(k)),
		)
		x.NoError(err)

		x.Len(plan.Tests, 1)
		x.Equal("parent", plan.Tests[0].Prop.Name())
		x.Equal(ormpatch.TestEqual, plan.Tests[0].Want)
		x.Equal(k, plan.Tests[0].Value.Interface())
	}))

	t.Run("nest shares the edge prefix", WithEntity(func(x *require.Assertions, e graph.Entity) {
		k := []byte("0123456789abcdef")
		plan, err := compile(x, e,
			patch.Target(patch.Name("parent")).Nest(
				patch.Target(patch.Name("id")).Assign(patch.Bytes(k)),
			),
		)
		x.NoError(err)

		x.Len(plan.Writes, 1)
		x.IsType(ormpatch.SetEdge{}, plan.Writes[0].Op)
	}))

	// The proto kind says bytes and the schema says UUID, and only the schema
	// knows how many. Refusing here keeps a compiled plan honest: a backend
	// that receives one should not have to re-check that a write is storable.
	t.Run("a UUID of the wrong width is refused", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("id")).In(patch.Name("parent")).
			Assign(patch.Bytes([]byte{1, 2, 3})))

		var bad *ormpatch.InvalidValueError
		x.ErrorAs(err, &bad)
		x.ErrorContains(err, "16 bytes, got 3")
		x.NotErrorIs(err, ormpatch.ErrUnsupported, "the engine is fine; the value is not")
	}))

	t.Run("a test against a UUID of the wrong width is refused too", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("id")).In(patch.Name("parent")).
			Test(patch.Bytes([]byte{1, 2, 3})))

		var bad *ormpatch.InvalidValueError
		x.ErrorAs(err, &bad)
	}))

	t.Run("assigning the edge itself is refused with a pointer to the fix", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("parent")).Assign(patch.Msg(
			patch.F(patch.Name("id"), patch.Bytes([]byte("0123456789abcdef"))),
		)))
		x.ErrorIs(err, ormpatch.ErrUnsupported)
		x.ErrorContains(err, "parent.id")
	}))

	t.Run("a non-key field of the target is refused", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e,
			patch.Target(patch.Name("name")).In(patch.Name("parent")).Assign(patch.Str("x")),
		)
		x.ErrorIs(err, ormpatch.ErrUnsupported)
	}))
}

func TestCompileMap(t *testing.T) {
	t.Run("assign one entry leaves the rest alone", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.MapStr("env")).In(patch.Name("labels")).Assign(patch.Str("prod")),
		)
		x.NoError(err)

		x.Empty(plan.Tests, "creating an entry cannot miss, so it needs no guard")
		x.Len(plan.Writes, 1)

		op, ok := plan.Writes[0].Op.(ormpatch.EditJSON)
		x.True(ok, "op is %T", plan.Writes[0].Op)
		x.Len(op.Ops, 1)
		x.Equal(ormpatch.JSONSet, op.Ops[0].Kind)
		x.Equal("env", op.Ops[0].Key.Interface())
		x.Equal("prod", op.Ops[0].Value.Interface())
	}))

	t.Run("removing an entry guards that it is there", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.MapStr("env")).In(patch.Name("labels")).Remove(),
		)
		x.NoError(err)

		// The miss the format demands is reported by the statement matching no
		// row rather than by a lookup.
		x.Len(plan.Tests, 1)
		x.Equal(ormpatch.TestExists, plan.Tests[0].Want)
		x.True(plan.Tests[0].HasKey)

		x.Len(plan.Writes, 1)
		x.Equal(ormpatch.JSONRemove, plan.Writes[0].Op.(ormpatch.EditJSON).Ops[0].Kind)
	}))

	t.Run("under SKIP the guard is dropped", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.MapStr("env")).In(patch.Name("labels")).Skip().Remove(),
		)
		x.NoError(err)

		x.Empty(plan.Tests, "the author asked for a miss to be tolerated")
		x.Len(plan.Writes, 1)
	}))

	t.Run("edits to one column compose in order", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.MapStr("a")).In(patch.Name("labels")).Assign(patch.Str("1")),
			patch.Target(patch.MapStr("b")).In(patch.Name("labels")).Assign(patch.Str("2")),
		)
		x.NoError(err)

		x.Len(plan.Writes, 1)
		x.Len(plan.Writes[0].Op.(ormpatch.EditJSON).Ops, 2)
	}))

	t.Run("every entry removed empties the column", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.EveryEntry()).In(patch.Name("labels")).Remove(),
		)
		x.NoError(err)

		x.Len(plan.Writes, 1)
		x.Equal(ormpatch.JSONClear, plan.Writes[0].Op.(ormpatch.EditJSON).Ops[0].Kind)
	}))

	t.Run("container remove empties the column", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Container().In(patch.Name("labels")).Remove())
		x.NoError(err)

		x.Len(plan.Writes, 1)
		x.Equal(ormpatch.JSONClear, plan.Writes[0].Op.(ormpatch.EditJSON).Ops[0].Kind)
	}))

	t.Run("a map key of the wrong type is a format error", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e,
			patch.Target(patch.MapInt(1)).In(patch.Name("labels")).Assign(patch.Str("x")),
		)
		x.Equal(patch.CodeIllegalArm, patch.CodeOf(err))
	}))
}

func TestCompileOrder(t *testing.T) {
	t.Run("a test after a write to the same column is refused", WithEntity(func(x *require.Assertions, e graph.Entity) {
		// The format says the test observes the assign; one statement's WHERE
		// sees the row as it was. Rather than emit a different meaning, refuse.
		_, err := compile(x, e,
			patch.Target(patch.Name("name")).Assign(patch.Str("new")),
			patch.Target(patch.Name("name")).Test(patch.Str("new")),
		)
		x.ErrorIs(err, ormpatch.ErrUnsupported)
	}))

	t.Run("a test before a write to the same column is fine", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.Name("name")).Test(patch.Str("old")),
			patch.Target(patch.Name("name")).Assign(patch.Str("new")),
		)
		x.NoError(err)

		x.Len(plan.Tests, 1)
		x.Len(plan.Writes, 1)
	}))

	t.Run("a test on a different column is fine", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			patch.Target(patch.Name("name")).Assign(patch.Str("new")),
			patch.Target(patch.Name("alias")).Test(patch.Str("ada")),
		)
		x.NoError(err)

		x.Len(plan.Tests, 1)
		x.Len(plan.Writes, 1)
	}))
}

func TestCompileDocument(t *testing.T) {
	t.Run("message_type must name the entity", WithEntity(func(x *require.Assertions, e graph.Entity) {
		p, err := patch.New("library.NotUser", patch.Target(patch.Name("name")).Assign(patch.Str("x")))
		x.NoError(err)

		_, err = ormpatch.Compile(e, p)
		x.Equal(patch.CodeMessageTypeMismatch, patch.CodeOf(err))
	}))

	t.Run("an untyped document applies", WithEntity(func(x *require.Assertions, e graph.Entity) {
		p := patch.MustNewUntyped(patch.Target(patch.Name("name")).Assign(patch.Str("x")))

		plan, err := ormpatch.Compile(e, p)
		x.NoError(err)
		x.Len(plan.Writes, 1)
	}))

	t.Run("structure is checked before identity", WithEntity(func(x *require.Assertions, e graph.Entity) {
		// A document that is both malformed and misaddressed reports the
		// structural code, matching the reference engine's order.
		p := &patchpb.Patch{}
		p.SetMessageType("library.NotUser")
		p.SetDelta(&patchpb.Delta{})

		_, err := ormpatch.Compile(e, p)
		x.Equal(patch.CodeEmptyCollection, patch.CodeOf(err))
	}))

	t.Run("an empty plan is visible", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("name")).Test(patch.Str("x")))
		x.NoError(err)

		// A test-only document writes nothing. A backend that read "no rows
		// updated" as "no such row" would be wrong about a row that is there.
		x.Empty(plan.Writes)
		x.False(plan.IsEmpty())
	}))
}

func TestCompileRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   func() patch.Op
	}{
		{"container scope at the root", func() patch.Op {
			return patch.Container().Remove()
		}},
		{"move", func() patch.Op {
			return patch.Target(patch.Name("name")).Move(patch.Here(patch.Name("desc")))
		}},
		{"copy", func() patch.Op {
			return patch.Target(patch.Name("name")).Copy(patch.Here(patch.Name("desc")))
		}},
		{"insert on a column", func() patch.Op {
			return patch.Target(patch.Name("name")).Insert(patch.Str("x"))
		}},
		{"oneof_member", func() patch.Op {
			return patch.Target(patch.Oneof("whatever")).Remove()
		}},
	} {
		t.Run(tc.name, WithEntity(func(x *require.Assertions, e graph.Entity) {
			_, err := compile(x, e, tc.op())
			x.Error(err)
			x.ErrorIs(err, ormpatch.ErrUnsupported,
				"a refusal must say it is this engine's limit, not the document's fault")
		}))
	}
}

func TestDeclaredDivergences(t *testing.T) {
	x := require.New(t)

	x.NotEmpty(ormpatch.DeclaredDivergences)
	for i, d := range ormpatch.DeclaredDivergences {
		x.NotEmpty(d.Construct, "divergence %d has no construct", i)
		x.NotEmpty(d.Cause, "divergence %d has no cause", i)
	}
}

var _ protoreflect.FieldNumber // keep the import honest if tests are trimmed
