package ormpatch_test

import (
	"testing"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"github.com/lesomnus/protobuf-patch/patchproto"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpatch"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// This is the test the design rests on.
//
// A stored delta is worth keeping only if it means the same thing to whoever
// reads it later. This engine compiles a document into column writes; the
// reference engine applies the same document to a message. An audit log that
// stores deltas and replays them onto a snapshot uses both, so where the two
// disagree the log lies.
//
// So: run the document through both and require the same answer. `apply` below
// interprets a [ormpatch.Plan] against a message -- it is what a storage
// backend does, with a message standing in for the row.

// apply executes a plan against a message, the way a backend executes it
// against a row.
//
// Tests are evaluated first and all together, since a backend issues them as
// one WHERE clause: if any fails, no row matches and nothing is written.
func apply(m proto.Message, plan *ormpatch.Plan) (proto.Message, bool, error) {
	out := proto.Clone(m)
	r := out.ProtoReflect()

	for _, t := range plan.Tests {
		ok, err := holds(r, t)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return m, false, nil // no row matched
		}
	}

	for _, w := range plan.Writes {
		if err := write(r, w); err != nil {
			return nil, false, err
		}
	}

	return out, true, nil
}

func holds(r protoreflect.Message, t ormpatch.Test) (bool, error) {
	fd := t.Prop.Descriptor()

	if ed, ok := t.Prop.(graph.Edge); ok {
		// The column holds the target's key, so the message stands in for the
		// row by carrying the key inside the edge message.
		if !r.Has(fd) {
			return t.Want == ormpatch.TestAbsent, nil
		}
		kd := ed.Target().Key().Descriptor()
		got := r.Get(fd).Message().Get(kd)
		switch t.Want {
		case ormpatch.TestExists:
			return true, nil
		case ormpatch.TestAbsent:
			return false, nil
		}
		return equal(got, t.Value), nil
	}

	switch {
	case t.HasKey:
		mp := r.Get(fd).Map()
		if !mp.Has(t.Key) {
			return t.Want == ormpatch.TestAbsent, nil
		}
		if t.Want != ormpatch.TestEqual {
			return t.Want == ormpatch.TestExists, nil
		}
		return equal(mp.Get(t.Key), t.Value), nil

	case t.HasIndex:
		l := r.Get(fd).List()
		i := int(t.Index)
		if i < 0 {
			i += l.Len()
		}
		if i < 0 || i >= l.Len() {
			return t.Want == ormpatch.TestAbsent, nil
		}
		if t.Want != ormpatch.TestEqual {
			return t.Want == ormpatch.TestExists, nil
		}
		return equal(l.Get(i), t.Value), nil
	}

	switch t.Want {
	case ormpatch.TestExists:
		return r.Has(fd), nil
	case ormpatch.TestAbsent:
		return !r.Has(fd), nil
	}
	return equal(r.Get(fd), t.Value), nil
}

func write(r protoreflect.Message, w ormpatch.Write) error {
	fd := w.Prop.Descriptor()

	switch op := w.Op.(type) {
	case ormpatch.SetColumn:
		setColumn(r, fd, op.Value)

	case ormpatch.ClearColumn:
		r.Clear(fd)

	case ormpatch.ClearEdge:
		r.Clear(fd)

	case ormpatch.SetEdge:
		ed := w.Prop.(graph.Edge)
		kd := ed.Target().Key().Descriptor()
		em := r.Mutable(fd).Message()
		em.Set(kd, op.Key)

	case ormpatch.EditJSON:
		for _, o := range op.Ops {
			switch o.Kind {
			case ormpatch.JSONClear:
				r.Clear(fd)
			case ormpatch.JSONSet:
				if o.HasKey {
					r.Mutable(fd).Map().Set(o.Key, o.Value)
					continue
				}
				l := r.Mutable(fd).List()
				i := int(o.Index)
				if i < 0 {
					i += l.Len()
				}
				l.Set(i, o.Value)
			case ormpatch.JSONRemove:
				if o.HasKey {
					r.Mutable(fd).Map().Clear(o.Key)
					continue
				}
				l := r.Mutable(fd).List()
				i := int(o.Index)
				if i < 0 {
					i += l.Len()
				}
				// Splice, the way removing from a list shrinks it.
				for j := i; j+1 < l.Len(); j++ {
					l.Set(j, l.Get(j+1))
				}
				l.Truncate(l.Len() - 1)
			case ormpatch.JSONAppend:
				r.Mutable(fd).List().Append(o.Value)
			}
		}
	}

	return nil
}

// setColumn writes a whole-column value.
//
// A collection in a Plan is described, not owned: it may be backed by
// dynamicpb, so it is copied into whatever the target uses rather than
// assigned. A SQL backend does the same when it walks the value to build JSON.
func setColumn(r protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value) {
	switch {
	case fd.IsMap():
		r.Clear(fd)
		dst := r.Mutable(fd).Map()
		v.Map().Range(func(k protoreflect.MapKey, e protoreflect.Value) bool {
			dst.Set(k, e)
			return true
		})
	case fd.IsList():
		r.Clear(fd)
		dst := r.Mutable(fd).List()
		src := v.List()
		for i := range src.Len() {
			dst.Append(src.Get(i))
		}
	default:
		r.Set(fd, v)
	}
}

func equal(a protoreflect.Value, b protoreflect.Value) bool {
	am, ok := a.Interface().(protoreflect.Message)
	if ok {
		bm, ok := b.Interface().(protoreflect.Message)
		return ok && proto.Equal(am.Interface(), bm.Interface())
	}
	switch av := a.Interface().(type) {
	case []byte:
		bv, ok := b.Interface().([]byte)
		return ok && string(av) == string(bv)
	default:
		return a.Interface() == b.Interface()
	}
}

// agree runs a document through both engines and requires the same result.
func agree(t *testing.T, seed *library.User, ops ...patch.Op) {
	t.Helper()
	x := require.New(t)

	g := graph.NewGraph()
	x.NoError(graph.Parse(t.Context(), g, library.File_library_user_proto))
	e := g.Entities[library.File_library_user_proto.FullName().Append("User")]

	p, err := patch.New(string(e.FullName()), ops[0], ops[1:]...)
	x.NoError(err)

	want, wantErr := patchproto.Apply(seed, p)

	plan, err := ormpatch.Compile(e, p)
	x.NoError(err, "compiling a document the reference engine accepts")

	got, matched, err := apply(proto.Clone(seed), plan)
	x.NoError(err)

	if wantErr != nil {
		// The reference aborts; this engine's statement must match no row.
		x.False(matched, "reference failed with %v but the plan applied", wantErr)
		return
	}

	x.True(matched, "reference applied but the plan matched no row")
	x.Empty(protoDiff(want, got), "engines disagree")
}

func protoDiff(want proto.Message, got proto.Message) string {
	if proto.Equal(want, got) {
		return ""
	}
	return "want:\n" + prototext(want) + "\ngot:\n" + prototext(got)
}

func prototext(m proto.Message) string {
	b, err := proto.Marshal(m)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func ptr[T any](v T) *T { return &v }

func seedUser() *library.User {
	return library.User_builder{
		Id:    []byte("0123456789abcdef"),
		Alias: ptr("ada"),
		Name:  ptr("Ada Lovelace"),
		Desc:  ptr("first programmer"),
		Labels: map[string]string{
			"env":  "prod",
			"team": "infra",
		},
	}.Build()
}

func TestEnginesAgree(t *testing.T) {
	t.Run("assign a scalar", func(t *testing.T) {
		agree(t, seedUser(), patch.Target(patch.Name("name")).Assign(patch.Str("Grace")))
	})

	t.Run("remove a scalar", func(t *testing.T) {
		agree(t, seedUser(), patch.Target(patch.Name("desc")).Remove())
	})

	t.Run("assign several columns", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.Name("name")).Assign(patch.Str("Grace")),
			patch.Target(patch.Name("desc")).Assign(patch.Str("rear admiral")),
		)
	})

	t.Run("a test that holds", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.Name("alias")).Test(patch.Str("ada")),
			patch.Target(patch.Name("name")).Assign(patch.Str("Grace")),
		)
	})

	t.Run("a test that does not hold abandons the document", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.Name("alias")).Test(patch.Str("nobody")),
			patch.Target(patch.Name("name")).Assign(patch.Str("Grace")),
		)
	})

	t.Run("set one map entry", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("env")).In(patch.Name("labels")).Assign(patch.Str("staging")),
		)
	})

	t.Run("create one map entry", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("tier")).In(patch.Name("labels")).Assign(patch.Str("gold")),
		)
	})

	t.Run("remove one map entry", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("env")).In(patch.Name("labels")).Remove(),
		)
	})

	t.Run("removing an absent entry abandons the document", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("nope")).In(patch.Name("labels")).Remove(),
		)
	})

	t.Run("removing an absent entry under SKIP is a no-op", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("nope")).In(patch.Name("labels")).Skip().Remove(),
		)
	})

	t.Run("two entries of one map", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("a")).In(patch.Name("labels")).Assign(patch.Str("1")),
			patch.Target(patch.MapStr("team")).In(patch.Name("labels")).Remove(),
		)
	})

	t.Run("empty the map", func(t *testing.T) {
		agree(t, seedUser(), patch.Container().In(patch.Name("labels")).Remove())
	})

	t.Run("replace the map wholesale", func(t *testing.T) {
		agree(t, seedUser(), patch.Container().In(patch.Name("labels")).Assign(
			patch.Map(patch.E(patch.MapStr("only"), patch.Str("one"))),
		))
	})

	t.Run("repoint an edge by its key", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.Name("id")).In(patch.Name("parent")).
				InOrCreate(patch.Name("parent")).Assign(patch.Bytes([]byte("fedcba9876543210"))),
		)
	})

	t.Run("a map entry test that holds", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("env")).In(patch.Name("labels")).Test(patch.Str("prod")),
			patch.Target(patch.Name("name")).Assign(patch.Str("Grace")),
		)
	})

	t.Run("a map entry test that does not hold", func(t *testing.T) {
		agree(t, seedUser(),
			patch.Target(patch.MapStr("env")).In(patch.Name("labels")).Test(patch.Str("dev")),
			patch.Target(patch.Name("name")).Assign(patch.Str("Grace")),
		)
	})
}

// A refusal must never be silent: whatever this engine declines, it declines
// loudly, so a caller never believes a document applied when it did not.
func TestRefusalsAreLoud(t *testing.T) {
	x := require.New(t)

	g := graph.NewGraph()
	x.NoError(graph.Parse(t.Context(), g, library.File_library_user_proto))
	e := g.Entities[library.File_library_user_proto.FullName().Append("User")]

	for _, ops := range [][]patch.Op{
		{patch.Target(patch.Name("name")).Move(patch.Here(patch.Name("desc")))},
		{patch.Container().Remove()},
		{patch.Target(patch.Name("metadata")).Assign(patch.Str("x"))},
	} {
		p, err := patch.New(string(e.FullName()), ops[0], ops[1:]...)
		x.NoError(err)

		plan, err := ormpatch.Compile(e, p)
		x.Error(err, "a refused document must not produce a plan")
		x.Nil(plan)
	}
}

var _ = patchpb.Patch{}
