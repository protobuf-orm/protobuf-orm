package ormpatch_test

import (
	"context"
	"testing"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/ormpatch"
	"github.com/stretchr/testify/require"
)

// WithList resolves graphtest.ValueField, which carries repeated scalars --
// library.User's only repeated prop is an edge, which is refused for other
// reasons and so cannot exercise an index.
func WithList(f func(x *require.Assertions, e graph.Entity)) func(*testing.T) {
	return func(t *testing.T) {
		x := require.New(t)

		g := graph.NewGraph()
		x.NoError(graph.Parse(context.Background(), g, graphtest.File_graphtest_field_proto))

		e, ok := g.Entities[graphtest.File_graphtest_field_proto.FullName().Append("ValueField")]
		x.True(ok, "entities: %v", g.Entities)

		f(x, e)
	}
}

func elem(i int64) *patch.TargetScope {
	return patch.Target(patch.Index(i)).In(patch.Name("implicit_i32s"))
}

// An index is a position in the list as the entries before it left it. One
// statement cannot honor that: the edits nest and apply in order, but the guard
// that stops an out-of-range write from landing somewhere else is a predicate,
// and a predicate reads the row as it was.
//
// So the rule is not "one index operation per document" -- it is that nothing
// may address a position after the list has grown or shrunk.
func TestCompileListIndex(t *testing.T) {
	t.Run("two writes that keep the length are fine", WithList(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			elem(0).Assign(patch.Int32(1)),
			elem(2).Assign(patch.Int32(3)),
		)
		x.NoError(err)
		x.Len(plan.Writes, 1, "both edits fold into the one column")
	}))

	t.Run("a remove last is fine", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e,
			elem(1).Assign(patch.Int32(1)),
			elem(0).Remove(),
		)
		x.NoError(err)
	}))

	// This one used to be refused, and is the reason Compile normalizes first.
	// The document says "drop the first, then overwrite what is now third",
	// which one statement cannot guard -- but it says the same thing as
	// "overwrite the fourth, then drop the first", and that one it can.
	t.Run("an index after a remove is reordered rather than refused", WithList(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e,
			elem(0).Remove(),
			elem(2).Assign(patch.Int32(9)),
		)
		x.NoError(err)
		x.Len(plan.Writes, 1)

		// The write is one column, edited twice, in the order that leaves the
		// same list behind.
		ops := plan.Writes[0].Op.(ormpatch.EditJSON).Ops
		x.Len(ops, 2)
		x.Equal(ormpatch.JSONSet, ops[0].Kind)
		x.EqualValues(3, ops[0].Index, "the assign moved up by the element the remove takes")
		x.Equal(ormpatch.JSONRemove, ops[1].Kind)
		x.EqualValues(0, ops[1].Index)
	}))

	// What normalization cannot prove, Compile still refuses. An append lands
	// at the old length, so the one index it moves is the one only the row
	// knows.
	t.Run("an index after a remove it cannot reorder is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e,
			elem(0).Remove(),
			elem(1).Remove(),
		)
		x.ErrorIs(err, ormpatch.ErrUnsupported)
		x.ErrorContains(err, "resized")
	}))

	t.Run("an index after an append is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e,
			patch.Target(patch.Append()).In(patch.Name("implicit_i32s")).Insert(patch.Int32(4)),
			elem(2).Assign(patch.Int32(9)),
		)
		x.ErrorIs(err, ormpatch.ErrUnsupported)
	}))

	t.Run("an index after a clear is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e,
			patch.Target(patch.Name("implicit_i32s")).Remove(),
			elem(0).Assign(patch.Int32(9)),
		)
		x.ErrorIs(err, ormpatch.ErrUnsupported)
	}))

	// A test reads a position too, so it is refused on the same grounds.
	t.Run("a test after a remove is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, elem(0).Remove(), elem(1).Test(patch.Int32(2)))
		x.ErrorIs(err, ormpatch.ErrUnsupported)
	}))

	// Appending twice names no position at all, so nothing is ambiguous.
	t.Run("appends do not need positions", WithList(func(x *require.Assertions, e graph.Entity) {
		app := patch.Target(patch.Append()).In(patch.Name("implicit_i32s"))
		_, err := compile(x, e, app.Insert(patch.Int32(1)), app.Insert(patch.Int32(2)))
		x.NoError(err)
	}))
}

// A column that cannot hold NULL has no absence to return to.
//
// This used to compile, render as SET col = NULL, and come back from the driver
// as a constraint violation -- an error no client could have predicted from its
// own document, arriving as an untyped code.
func TestCompileClearColumn(t *testing.T) {
	t.Run("clearing a nullable column is fine", WithList(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("nullable_string")).Remove())
		x.NoError(err)
		x.Len(plan.Writes, 1)
		x.IsType(ormpatch.ClearColumn{}, plan.Writes[0].Op)
	}))

	t.Run("clearing a column that is not nullable is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("implicit_string")).Remove())
		x.ErrorIs(err, ormpatch.ErrUnsupported)
		x.ErrorContains(err, "not nullable")
	}))

	// The value it would have written is still reachable; it just has to be
	// asked for, because it is a different request.
	t.Run("assigning the zero is still fine", WithList(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("implicit_string")).Assign(patch.Str("")))
		x.NoError(err)
		x.Len(plan.Writes, 1)
	}))
}

func TestCompileListBounds(t *testing.T) {
	t.Run("a negative index is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, elem(-1).Assign(patch.Int32(9)))
		x.ErrorIs(err, ormpatch.ErrUnsupported)
		x.ErrorContains(err, "counts from the end")
	}))

	// It wraps rather than missing in at least one backend, and its own
	// out-of-range guard wraps with it -- so the write lands somewhere the
	// document never named and nothing reports the substitution.
	t.Run("an index past the addressable range is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, elem(1<<31).Assign(patch.Int32(9)))
		x.ErrorIs(err, ormpatch.ErrUnsupported)
	}))

	t.Run("the largest addressable index compiles", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, elem(1<<31-1).Assign(patch.Int32(9)))
		x.NoError(err)
	}))
}

// The column holds one serialization of the collection, so comparing it whole
// asks whether two spellings match rather than whether two collections do.
func TestCompileWholeCollectionTest(t *testing.T) {
	t.Run("comparing a whole list is refused", WithList(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("implicit_i32s")).
			Test(patch.List(patch.Int32(1), patch.Int32(2))))
		x.ErrorIs(err, ormpatch.ErrUnsupported)
		x.ErrorContains(err, "Test an entry")
	}))

	t.Run("comparing a whole map is refused", WithEntity(func(x *require.Assertions, e graph.Entity) {
		_, err := compile(x, e, patch.Target(patch.Name("labels")).
			Test(patch.Map(patch.E(patch.MapStr("a"), patch.Str("1")))))
		x.ErrorIs(err, ormpatch.ErrUnsupported)
	}))

	// Asking whether the column is set is a different question, and the
	// database answers it natively.
	t.Run("asking whether the column is set is fine", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("labels")).Exists(true))
		x.NoError(err)
		x.Len(plan.Tests, 1)
	}))

	t.Run("comparing one entry is fine", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.MapStr("a")).In(patch.Name("labels")).
			Test(patch.Str("1")))
		x.NoError(err)
		x.Len(plan.Tests, 1)
	}))

	// A whole-collection ASSIGN is untouched: writing a serialization is
	// exactly what the column is for.
	t.Run("assigning a whole map is still fine", WithEntity(func(x *require.Assertions, e graph.Entity) {
		plan, err := compile(x, e, patch.Target(patch.Name("labels")).
			Assign(patch.Map(patch.E(patch.MapStr("a"), patch.Str("1")))))
		x.NoError(err)
		x.Len(plan.Writes, 1)
	}))
}
