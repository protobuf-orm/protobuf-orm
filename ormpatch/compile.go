package ormpatch

import (
	"fmt"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Compile resolves a patch document against an entity and returns the writes
// and predicates one UPDATE needs.
//
// It never reads storage: an edge is repointed from the key the document
// carries, and a missing target becomes a predicate rather than a lookup. The
// result is therefore a pure function of the entity's schema and the document.
func Compile(e graph.Entity, p *patchpb.Patch) (*Plan, error) {
	return CompileWith(e, p, patch.DefaultLimits)
}

// CompileWith is [Compile] under caller-chosen limits. A zero field falls back
// to the corresponding [patch.DefaultLimits] value.
func CompileWith(e graph.Entity, p *patchpb.Patch, lim patch.Limits) (*Plan, error) {
	if e == nil {
		return nil, patch.Errf(patch.CodeMissingField, "", "no entity")
	}
	if p == nil {
		return nil, patch.Errf(patch.CodeMissingField, "", "no patch")
	}

	// Structure before identity, matching the reference engine: a document that
	// is both malformed and misaddressed reports the structural code.
	if err := patch.ValidateWith(p, lim); err != nil {
		return nil, err
	}
	if t := p.GetMessageType(); t != "" {
		if protoreflect.FullName(t) != e.FullName() {
			return nil, patch.Errf(patch.CodeMessageTypeMismatch, "message_type",
				"document is for %s, entity is %s", t, e.FullName())
		}
	}

	c := &compiler{
		props:  indexProps(e),
		writes: map[protoreflect.FieldNumber]*Write{},
		wrote:  map[protoreflect.FieldNumber]patch.At{},
	}
	if err := c.delta(p.GetDelta(), site{kind: siteRoot}, patch.At("delta"), 0); err != nil {
		return nil, err
	}

	plan := &Plan{Entity: e, Tests: c.tests}
	for _, n := range c.order {
		plan.Writes = append(plan.Writes, *c.writes[n])
	}

	return plan, nil
}

type siteKind int

const (
	// siteRoot is the entity itself: its columns are the targets.
	siteRoot siteKind = iota
	// siteEdge is a foreign-key column, reached by a path through the edge.
	siteEdge
	// siteMap is a JSON object column.
	siteMap
	// siteList is a JSON array column.
	siteList
)

// site is the container an entry addresses, once its path has been resolved.
type site struct {
	kind siteKind
	prop graph.Prop // nil at siteRoot
}

type compiler struct {
	props *props

	tests  []Test
	order  []protoreflect.FieldNumber
	writes map[protoreflect.FieldNumber]*Write

	// wrote records where a column was first written, so that a later read of
	// it can be refused with the position that made it unreadable.
	wrote map[protoreflect.FieldNumber]patch.At
}

// maxNest bounds how deep `nest` may chain here. It matches
// patch.DefaultLimits.NestDepth; ValidateWith has already enforced it over the
// document, and this is only a guard on the recursion.
const maxNest = 64

func (c *compiler) delta(d *patchpb.Delta, s site, at patch.At, depth int) error {
	if depth > maxNest {
		return patch.Errf(patch.CodeTooDeep, at, "nested too deeply")
	}
	for i, e := range d.GetEntries() {
		if err := c.entry(e, s, at.Index("entries", i), depth); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) entry(e *patchpb.Entry, s site, at patch.At, depth int) error {
	s, err := c.walk(s, e.GetPath().GetSegments(), at.Sub("path"))
	if err != nil {
		return err
	}

	switch e.WhichScope() {
	case patchpb.Entry_Container_case:
		return c.container(e, s, at, depth)
	case patchpb.Entry_Targets_case:
		return c.targets(e, s, at, depth)
	}

	// Validate refuses an unset scope, so this is an arm from a newer revision.
	return patch.Errf(patch.CodeMissingOneof, at, "unrecognized scope")
}

// walk resolves an entry's path into the container it names.
//
// A row is one level deep, so at most one segment can be followed: into an
// edge, whose foreign key is a column, or into a map or list, whose entries are
// addressable inside one JSON column. Anything further has nowhere to go.
func (c *compiler) walk(s site, segs []*patchpb.Key, at patch.At) (site, error) {
	for i, seg := range segs {
		at := at.Index("segments", i)

		if s.kind != siteRoot {
			return s, unsupportedf(at,
				"a path cannot go deeper than one column; %s is already inside %s",
				at, s.prop.Name())
		}
		if seg.WhichKind() != patchpb.Key_Field_case {
			// The root container is a message, so only a field key can enter it.
			return s, patch.Errf(patch.CodePathNotReached, at,
				"the entity is a message; a field key is required")
		}

		v, vacant, err := c.props.resolve(seg.GetField(), at.Sub("field"))
		if err != nil {
			return s, err
		}
		if vacant {
			// A path never tolerates a miss, whatever on_missing says.
			return s, patch.Errf(patch.CodePathNotReached, at.Sub("field"),
				"%s declares no such field", c.props.ent.FullName())
		}

		switch {
		case isMap(v):
			s = site{kind: siteMap, prop: v}
		case isList(v):
			s = site{kind: siteList, prop: v}
		default:
			if ed, ok := v.(graph.Edge); ok {
				s = site{kind: siteEdge, prop: ed}
				continue
			}
			return s, unsupportedf(at,
				"%s is a single column; there is nothing inside it to address",
				v.Name())
		}
	}

	return s, nil
}

// container handles an entry whose scope is the container its path reached.
func (c *compiler) container(e *patchpb.Entry, s site, at patch.At, depth int) error {
	switch s.kind {
	case siteRoot:
		return unsupportedf(at,
			"container scope at the root addresses the whole entity; the row is "+
				"already identified by the request")
	case siteEdge:
		return unsupportedf(at,
			"container scope on edge %s addresses the target entity as a whole, "+
				"which this row does not hold", s.prop.Name())
	}

	fd := s.prop.Descriptor()

	switch e.WhichKind() {
	case patchpb.Entry_Remove_case:
		// Emptying a collection is a whole-column write.
		return c.write(s.prop, at, EditJSON{Ops: []JSONOp{{Kind: JSONClear, At: at}}})

	case patchpb.Entry_Assign_case:
		v, err := materialize(e.GetAssign().GetValue(), fd, patch.SiteField, at.Sub("assign").Sub("value"))
		if err != nil {
			return err
		}
		return c.write(s.prop, at, SetColumn{Value: v})

	case patchpb.Entry_Insert_case:
		if s.kind != siteList {
			return unsupportedf(at,
				"insert must not overwrite, so it has to know whether %s already "+
					"holds each key -- that is a read", s.prop.Name())
		}
		// Container-scope insert on a list appends.
		v, err := materialize(e.GetInsert().GetValue(), fd, patch.SiteField, at.Sub("insert").Sub("value"))
		if err != nil {
			return err
		}
		l := v.List()
		ops := make([]JSONOp, 0, l.Len())
		for i := range l.Len() {
			ops = append(ops, JSONOp{Kind: JSONAppend, Value: l.Get(i), HasValue: true, At: at})
		}
		return c.write(s.prop, at, EditJSON{Ops: ops})

	case patchpb.Entry_Nest_case:
		return c.delta(e.GetNest().GetDelta(), s, at.Sub("nest").Sub("delta"), depth+1)

	case patchpb.Entry_Test_case:
		return unsupportedf(at,
			"a container-scope test asserts non-emptiness, which this engine "+
				"does not compile; test an entry instead")

	case patchpb.Entry_Move_case, patchpb.Entry_Copy_case:
		// Validate already refuses these against a container.
		return patch.Errf(patch.CodeIllegalScope, at, "move and copy need targets")
	}

	return patch.Errf(patch.CodeMissingOneof, at, "unrecognized operation")
}

// targets handles an entry that names specific locations.
func (c *compiler) targets(e *patchpb.Entry, s site, at patch.At, depth int) error {
	sels := e.GetTargets().GetSelectors()
	at = at.Sub("targets")

	// A move or copy would have to read a column this same statement assigns.
	switch e.WhichKind() {
	case patchpb.Entry_Move_case, patchpb.Entry_Copy_case:
		return unsupportedf(at,
			"move and copy read their source after the targets are written, "+
				"which one statement cannot do")
	}

	if e.WhichKind() == patchpb.Entry_Nest_case {
		if len(sels) != 1 {
			return unsupportedf(at,
				"nest into %d targets would apply one delta to several "+
					"containers", len(sels))
		}
		inner, err := c.enter(s, sels[0], at.Index("selectors", 0))
		if err != nil {
			return err
		}
		return c.delta(e.GetNest().GetDelta(), inner, at.Sub("nest").Sub("delta"), depth+1)
	}

	seen := map[string]int{}
	for i, sel := range sels {
		at := at.Index("selectors", i)

		id, err := c.ident(s, sel, at)
		if err != nil {
			return err
		}
		if j, ok := seen[id]; ok {
			return patch.Errf(patch.CodeDuplicateTarget, at,
				"selector %d already names this location", j)
		}
		seen[id] = i

		if err := c.one(e, s, sel, at); err != nil {
			return err
		}
	}

	return nil
}

// enter resolves a selector to the container beneath it, for nest.
func (c *compiler) enter(s site, sel *patchpb.Selector, at patch.At) (site, error) {
	if s.kind != siteRoot {
		return s, unsupportedf(at, "nesting deeper than one column has nowhere to go")
	}
	if sel.WhichKind() != patchpb.Selector_Key_case || sel.GetKey().WhichKind() != patchpb.Key_Field_case {
		return s, patch.Errf(patch.CodeIllegalArm, at, "the entity is a message; a field key is required")
	}

	v, vacant, err := c.props.resolve(sel.GetKey().GetField(), at)
	if err != nil {
		return s, err
	}
	if vacant {
		return s, patch.Errf(patch.CodePathNotReached, at, "no such field")
	}

	switch {
	case isMap(v):
		return site{kind: siteMap, prop: v}, nil
	case isList(v):
		return site{kind: siteList, prop: v}, nil
	}
	if ed, ok := v.(graph.Edge); ok {
		return site{kind: siteEdge, prop: ed}, nil
	}

	return s, unsupportedf(at, "%s is a single column; nothing nests inside it", v.Name())
}

// ident names the location a selector resolves to, so that two selectors in one
// entry naming the same one can be refused.
func (c *compiler) ident(s site, sel *patchpb.Selector, at patch.At) (string, error) {
	switch s.kind {
	case siteRoot, siteEdge:
		if sel.WhichKind() != patchpb.Selector_Key_case || sel.GetKey().WhichKind() != patchpb.Key_Field_case {
			return "", nil // one() reports the real error
		}
		v, vacant, err := c.props.resolve(sel.GetKey().GetField(), at)
		if err != nil || vacant {
			return "", nil
		}
		return fmt.Sprintf("f%d", v.Number()), nil

	case siteMap:
		if sel.WhichKind() == patchpb.Selector_EveryEntry_case {
			return "every", nil
		}
		if sel.WhichKind() != patchpb.Selector_Key_case || sel.GetKey().WhichKind() != patchpb.Key_MapKey_case {
			return "", nil
		}
		mk, err := patch.MapKeyFor(sel.GetKey().GetMapKey(), s.prop.Descriptor(), at)
		if err != nil {
			return "", nil
		}
		return fmt.Sprintf("k%v", mk.Interface()), nil

	case siteList:
		switch sel.WhichKind() {
		case patchpb.Selector_Append_case:
			return "append", nil
		case patchpb.Selector_Key_case:
			if sel.GetKey().WhichKind() == patchpb.Key_Index_case {
				return fmt.Sprintf("i%d", sel.GetKey().GetIndex()), nil
			}
		}
	}

	return "", nil
}

// one compiles a single selector of an entry.
func (c *compiler) one(e *patchpb.Entry, s site, sel *patchpb.Selector, at patch.At) error {
	switch s.kind {
	case siteRoot:
		return c.atColumn(e, sel, at)
	case siteEdge:
		return c.atEdge(e, s, sel, at)
	case siteMap:
		return c.atMapEntry(e, s, sel, at)
	case siteList:
		return c.atListElem(e, s, sel, at)
	}
	return unsupportedf(at, "unreachable site")
}

// atColumn handles a target that is a column of the entity.
func (c *compiler) atColumn(e *patchpb.Entry, sel *patchpb.Selector, at patch.At) error {
	switch sel.WhichKind() {
	case patchpb.Selector_Range_case, patchpb.Selector_Append_case, patchpb.Selector_EveryEntry_case:
		return patch.Errf(patch.CodeIllegalArm, at,
			"the entity is a message, not a collection")
	case patchpb.Selector_OneofMember_case:
		return unsupportedf(at,
			"selecting whichever member of a oneof is set requires reading the row")
	}
	if sel.GetKey().WhichKind() != patchpb.Key_Field_case {
		return patch.Errf(patch.CodeIllegalArm, at, "the entity is a message; a field key is required")
	}

	v, vacant, err := c.props.resolve(sel.GetKey().GetField(), at)
	if err != nil {
		return err
	}
	if vacant {
		if e.GetOnMissing() == patchpb.OnMissing_ON_MISSING_SKIP && e.WhichKind() != patchpb.Entry_Test_case {
			return nil
		}
		if e.WhichKind() == patchpb.Entry_Test_case {
			// A test reads a missing target rather than failing on it, and the
			// only thing it can conclude is absence.
			return c.testAbsent(e, at)
		}
		return patch.Errf(patch.CodeVacantTarget, at, "no such field")
	}

	ed, isEdge := v.(graph.Edge)

	switch e.WhichKind() {
	case patchpb.Entry_Test_case:
		return c.test(e, v, nil, nil, at)

	case patchpb.Entry_Remove_case:
		if err := c.mutable(v, at); err != nil {
			return err
		}
		if isEdge {
			if !ed.IsNullable() {
				return unsupportedf(at,
					"edge %s is not nullable, so its foreign key cannot be cleared",
					ed.Name())
			}
			return c.write(v, at, ClearEdge{})
		}
		if graph.IsCollection(v) {
			return c.write(v, at, EditJSON{Ops: []JSONOp{{Kind: JSONClear, At: at}}})
		}
		return c.write(v, at, ClearColumn{})

	case patchpb.Entry_Assign_case:
		if err := c.mutable(v, at); err != nil {
			return err
		}
		if isEdge {
			return unsupportedf(at,
				"assigning edge %s would carry a whole %s where the row holds "+
					"only its key; address %s.%s instead",
				ed.Name(), ed.Target().Name(), ed.Name(), ed.Target().Key().Name())
		}
		pv, err := materialize(e.GetAssign().GetValue(), v.Descriptor(), patch.SiteField, at)
		if err != nil {
			return err
		}
		return c.write(v, at, SetColumn{Value: pv})

	case patchpb.Entry_Insert_case:
		return unsupportedf(at,
			"insert must not overwrite, so it has to know whether %s is already "+
				"set -- that is a read", v.Name())
	}

	return patch.Errf(patch.CodeMissingOneof, at, "unrecognized operation")
}

// atEdge handles a target reached by descending through an edge -- the (c)
// addressing described in the package doc.
func (c *compiler) atEdge(e *patchpb.Entry, s site, sel *patchpb.Selector, at patch.At) error {
	ed := s.prop.(graph.Edge)
	key := ed.Target().Key()

	if sel.WhichKind() != patchpb.Selector_Key_case || sel.GetKey().WhichKind() != patchpb.Key_Field_case {
		return patch.Errf(patch.CodeIllegalArm, at, "a field key is required")
	}

	fd, vacant, err := patch.ResolveField(ed.Target().Descriptor(), sel.GetKey().GetField(), at)
	if err != nil {
		return err
	}
	if vacant {
		return patch.Errf(patch.CodeVacantTarget, at, "%s declares no such field", ed.Target().FullName())
	}
	if fd.Number() != key.Number() {
		return unsupportedf(at,
			"edge %s is one foreign-key column, so only %s.%s is present in this "+
				"row; %s is not",
			ed.Name(), ed.Target().Name(), key.Name(), fd.Name())
	}

	switch e.WhichKind() {
	case patchpb.Entry_Test_case:
		return c.test(e, ed, nil, nil, at)

	case patchpb.Entry_Assign_case:
		if err := c.mutable(ed, at); err != nil {
			return err
		}
		pv, err := materialize(e.GetAssign().GetValue(), key.Descriptor(), patch.SiteField, at)
		if err != nil {
			return err
		}
		return c.write(ed, at, SetEdge{Key: pv})

	case patchpb.Entry_Remove_case:
		return unsupportedf(at,
			"removing %s.%s would clear the key of the entity %s points at; "+
				"remove %s itself to unset the edge",
			ed.Target().Name(), key.Name(), ed.Name(), ed.Name())
	}

	return unsupportedf(at, "only assign and test apply to an edge's key")
}

// atMapEntry handles a target inside a map column.
func (c *compiler) atMapEntry(e *patchpb.Entry, s site, sel *patchpb.Selector, at patch.At) error {
	fd := s.prop.Descriptor()

	if sel.WhichKind() == patchpb.Selector_EveryEntry_case {
		switch e.WhichKind() {
		case patchpb.Entry_Remove_case:
			if err := c.mutable(s.prop, at); err != nil {
				return err
			}
			return c.write(s.prop, at, EditJSON{Ops: []JSONOp{{Kind: JSONClear, At: at}}})
		case patchpb.Entry_Test_case:
			return unsupportedf(at, "testing every entry requires reading them")
		}
		return unsupportedf(at,
			"applying %s to every entry requires knowing what the keys are",
			kindName(e))
	}
	if sel.WhichKind() != patchpb.Selector_Key_case || sel.GetKey().WhichKind() != patchpb.Key_MapKey_case {
		return patch.Errf(patch.CodeIllegalArm, at, "%s is a map; a map key is required", s.prop.Name())
	}

	mk, err := patch.MapKeyFor(sel.GetKey().GetMapKey(), fd, at)
	if err != nil {
		return err
	}

	switch e.WhichKind() {
	case patchpb.Entry_Test_case:
		return c.test(e, s.prop, &mk, nil, at)

	case patchpb.Entry_Assign_case:
		if err := c.mutable(s.prop, at); err != nil {
			return err
		}
		// A map key with no entry is a slot a write fills, so an assign never
		// misses and needs no guard.
		pv, err := materialize(e.GetAssign().GetValue(), fd, patch.SiteMapValue, at)
		if err != nil {
			return err
		}
		return c.write(s.prop, at, EditJSON{Ops: []JSONOp{
			{Kind: JSONSet, Key: mk, HasKey: true, Value: pv, HasValue: true, At: at},
		}})

	case patchpb.Entry_Remove_case:
		if err := c.mutable(s.prop, at); err != nil {
			return err
		}
		// Removing an entry that is not there is a miss. The statement cannot
		// report it, so require it as a predicate: if the key is absent the
		// UPDATE matches no row and the whole document is abandoned, which is
		// what a miss means. Under SKIP the author asked for the opposite, and
		// deleting an absent key is already a no-op.
		if e.GetOnMissing() != patchpb.OnMissing_ON_MISSING_SKIP {
			c.tests = append(c.tests, Test{
				Prop: s.prop, Key: mk, HasKey: true, Want: TestExists, At: at,
			})
		}
		return c.write(s.prop, at, EditJSON{Ops: []JSONOp{
			{Kind: JSONRemove, Key: mk, HasKey: true, At: at},
		}})

	case patchpb.Entry_Insert_case:
		return unsupportedf(at,
			"insert must not overwrite, so it has to know whether the key is "+
				"already there -- that is a read")
	}

	return patch.Errf(patch.CodeMissingOneof, at, "unrecognized operation")
}

// atListElem handles a target inside a list column.
func (c *compiler) atListElem(e *patchpb.Entry, s site, sel *patchpb.Selector, at patch.At) error {
	fd := s.prop.Descriptor()

	switch sel.WhichKind() {
	case patchpb.Selector_Append_case:
		if e.WhichKind() != patchpb.Entry_Insert_case {
			return patch.Errf(patch.CodeIllegalSelector, at, "append belongs to insert")
		}
		if err := c.mutable(s.prop, at); err != nil {
			return err
		}
		pv, err := materialize(e.GetInsert().GetValue(), fd, patch.SiteElement, at)
		if err != nil {
			return err
		}
		return c.write(s.prop, at, EditJSON{Ops: []JSONOp{
			{Kind: JSONAppend, Value: pv, HasValue: true, At: at},
		}})

	case patchpb.Selector_Range_case:
		return unsupportedf(at,
			"a span names several elements whose positions shift as the "+
				"statement runs; address one index at a time")

	case patchpb.Selector_EveryEntry_case, patchpb.Selector_OneofMember_case:
		return patch.Errf(patch.CodeIllegalArm, at, "%s is a list", s.prop.Name())
	}

	if sel.GetKey().WhichKind() != patchpb.Key_Index_case {
		return patch.Errf(patch.CodeIllegalArm, at, "%s is a list; an index is required", s.prop.Name())
	}
	idx := sel.GetKey().GetIndex()

	switch e.WhichKind() {
	case patchpb.Entry_Test_case:
		return c.test(e, s.prop, nil, &idx, at)

	case patchpb.Entry_Assign_case, patchpb.Entry_Remove_case:
		if err := c.mutable(s.prop, at); err != nil {
			return err
		}
		// An index outside the list is a miss for every kind. Guard it the same
		// way a map key is guarded, so an out-of-range write abandons the
		// document instead of silently landing somewhere else.
		if e.GetOnMissing() != patchpb.OnMissing_ON_MISSING_SKIP {
			c.tests = append(c.tests, Test{
				Prop: s.prop, Index: idx, HasIndex: true, Want: TestExists, At: at,
			})
		}
		if e.WhichKind() == patchpb.Entry_Remove_case {
			return c.write(s.prop, at, EditJSON{Ops: []JSONOp{
				{Kind: JSONRemove, Index: idx, HasIndex: true, At: at},
			}})
		}
		pv, err := materialize(e.GetAssign().GetValue(), fd, patch.SiteElement, at)
		if err != nil {
			return err
		}
		return c.write(s.prop, at, EditJSON{Ops: []JSONOp{
			{Kind: JSONSet, Index: idx, HasIndex: true, Value: pv, HasValue: true, At: at},
		}})

	case patchpb.Entry_Insert_case:
		return unsupportedf(at,
			"inserting at an index splices the list, shifting every element "+
				"after it; append instead")
	}

	return patch.Errf(patch.CodeMissingOneof, at, "unrecognized operation")
}

// test records a predicate, refusing one that would read a column this document
// has already written.
func (c *compiler) test(e *patchpb.Entry, v graph.Prop, key *protoreflect.MapKey, idx *int64, at patch.At) error {
	if w, ok := c.wrote[v.Number()]; ok {
		return unsupportedf(at,
			"this reads %s, which %s already writes; entries observe each other "+
				"in the document but one statement evaluates every assignment "+
				"against the row as it was",
			v.Name(), w)
	}

	t := Test{Prop: v, At: at}
	if key != nil {
		t.Key, t.HasKey = *key, true
	}
	if idx != nil {
		t.Index, t.HasIndex = *idx, true
	}

	switch e.GetTest().WhichWant() {
	case patchpb.Test_Exists_case:
		if e.GetTest().GetExists() {
			t.Want = TestExists
		} else {
			t.Want = TestAbsent
		}
		c.tests = append(c.tests, t)
		return nil

	case patchpb.Test_Value_case:
		fd := v.Descriptor()
		site := patch.SiteField
		switch {
		case key != nil:
			site = patch.SiteMapValue
		case idx != nil:
			site = patch.SiteElement
		}
		if ed, ok := v.(graph.Edge); ok {
			// The column holds the target's key, so that is what to compare.
			fd, site = ed.Target().Key().Descriptor(), patch.SiteField
		}

		pv, err := materialize(e.GetTest().GetValue(), fd, site, at)
		if err != nil {
			return err
		}
		t.Want, t.Value = TestEqual, pv
		c.tests = append(c.tests, t)
		return nil
	}

	return patch.Errf(patch.CodeMissingOneof, at, "unrecognized test")
}

// testAbsent records the only conclusion a test can draw about a field the
// entity does not declare.
func (c *compiler) testAbsent(e *patchpb.Entry, at patch.At) error {
	if e.GetTest().WhichWant() == patchpb.Test_Exists_case && !e.GetTest().GetExists() {
		return nil // asserting absence of what does not exist always holds
	}
	return patch.Errf(patch.CodeTestFailed, at, "no such field")
}

// mutable refuses a write the schema does not allow.
func (c *compiler) mutable(v graph.Prop, at patch.At) error {
	if v.IsImmutable() {
		return &ImmutableError{At: at, Prop: v.Name()}
	}
	return nil
}

// write folds an operation into the column's pending write.
func (c *compiler) write(v graph.Prop, at patch.At, op Op) error {
	if _, ok := c.wrote[v.Number()]; !ok {
		c.wrote[v.Number()] = at
	}

	w, ok := c.writes[v.Number()]
	if !ok {
		c.order = append(c.order, v.Number())
		c.writes[v.Number()] = &Write{Prop: v, At: at, Op: op}
		return nil
	}

	// Two edits to the same JSON column compose in order.
	if prev, ok := w.Op.(EditJSON); ok {
		if next, ok := op.(EditJSON); ok {
			w.Op = EditJSON{Ops: append(prev.Ops, next.Ops...)}
			return nil
		}
	}

	// Anything else is a replacement: the later write wins, which is what
	// applying them in order would leave behind. A whole-column write after a
	// partial edit discards the edit, and a partial edit after a whole-column
	// write cannot be expressed in one assignment.
	if _, ok := op.(EditJSON); ok {
		return unsupportedf(at,
			"%s is edited in part after being written whole by %s; one statement "+
				"assigns a column once", v.Name(), w.At)
	}
	w.Op = op
	w.At = at

	return nil
}

func kindName(e *patchpb.Entry) string {
	switch e.WhichKind() {
	case patchpb.Entry_Remove_case:
		return "remove"
	case patchpb.Entry_Test_case:
		return "test"
	case patchpb.Entry_Insert_case:
		return "insert"
	case patchpb.Entry_Assign_case:
		return "assign"
	case patchpb.Entry_Move_case:
		return "move"
	case patchpb.Entry_Copy_case:
		return "copy"
	case patchpb.Entry_Nest_case:
		return "nest"
	}
	return "an unrecognized operation"
}
