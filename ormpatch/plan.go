package ormpatch

import (
	"fmt"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Plan is a compiled patch document: everything one UPDATE needs.
//
// Tests are predicates the row must already satisfy; Writes are the column
// assignments. A backend that issues them as one statement gets the document's
// atomicity for free, and each Test becomes a compare-and-swap.
//
// Writes carries at most one entry per column: entries writing the same column
// are folded in document order.
type Plan struct {
	Entity graph.Entity
	Tests  []Test
	Writes []Write
}

// IsEmpty reports whether the plan would issue no statement at all.
//
// It is worth checking: a document of nothing but tests is legal and means
// "assert this", and a backend that maps "no rows updated" to NotFound would
// otherwise report a missing row for one that is present.
func (p *Plan) IsEmpty() bool {
	return len(p.Tests) == 0 && len(p.Writes) == 0
}

// Write is one column's new value.
type Write struct {
	// Prop is the column. It is a [graph.Edge] exactly when Op is [SetEdge] or
	// [ClearEdge].
	Prop graph.Prop

	// At is where in the document this write came from, for error reporting.
	At patch.At

	Op Op
}

// Op is what to do to a column. Exactly one implementation applies per Write.
type Op interface {
	isOp()

	// Describe renders the operation for diagnostics.
	Describe() string
}

// SetColumn replaces the whole column.
//
// For a JSON column (a map, a list, or a TYPE_JSON field) this is a wholesale
// replacement, matching the format's rule that assigning a collection truncates
// before it fills.
type SetColumn struct{ Value protoreflect.Value }

// ClearColumn returns the column to absence: NULL where the prop is nullable,
// otherwise its zero -- the same distinction [graph.Prop.IsNullable] draws.
type ClearColumn struct{}

// SetEdge writes an edge's foreign key. Key is a value of the target entity's
// key field, already resolved -- compiling never consults storage.
type SetEdge struct{ Key protoreflect.Value }

// ClearEdge nulls an edge's foreign key. Only a nullable edge can take it.
type ClearEdge struct{}

// EditJSON changes part of a JSON column, leaving the rest of the document
// alone. Ops apply in order.
//
// This is the operation a request message cannot express, and the reason to
// compile a document at all: editing one map entry without reading the map
// first means two concurrent writers touching different entries do not lose
// each other's work.
type EditJSON struct{ Ops []JSONOp }

func (SetColumn) isOp()  {}
func (ClearColumn) isOp() {}
func (SetEdge) isOp()    {}
func (ClearEdge) isOp()  {}
func (EditJSON) isOp()   {}

func (o SetColumn) Describe() string  { return fmt.Sprintf("set %v", o.Value.Interface()) }
func (ClearColumn) Describe() string  { return "clear" }
func (o SetEdge) Describe() string    { return fmt.Sprintf("set edge %v", o.Key.Interface()) }
func (ClearEdge) Describe() string    { return "clear edge" }
func (o EditJSON) Describe() string   { return fmt.Sprintf("edit json (%d ops)", len(o.Ops)) }

// JSONOpKind is what a [JSONOp] does at its address.
type JSONOpKind int

const (
	// JSONSet writes a value at the address, creating a map entry if absent.
	JSONSet JSONOpKind = iota
	// JSONRemove deletes the address. A map entry disappears; a list element is
	// spliced out and the list shrinks.
	JSONRemove
	// JSONAppend adds a value to the end of a list.
	JSONAppend
	// JSONClear empties the whole document -- `remove` at container scope.
	JSONClear
)

func (k JSONOpKind) String() string {
	switch k {
	case JSONSet:
		return "set"
	case JSONRemove:
		return "remove"
	case JSONAppend:
		return "append"
	case JSONClear:
		return "clear"
	}
	return fmt.Sprintf("JSONOpKind(%d)", int(k))
}

// JSONOp is one change inside a JSON column.
type JSONOp struct {
	Kind JSONOpKind

	// Key addresses a map entry. Valid when the column is a map and Kind is
	// JSONSet or JSONRemove.
	Key protoreflect.MapKey
	// HasKey distinguishes an addressed entry from the whole document, since
	// the zero MapKey is a legitimate key.
	HasKey bool

	// Index addresses a list element, already normalized against the list's
	// length at compile time is NOT possible -- the length lives in the row --
	// so it is the document's index verbatim. A negative index counts from the
	// end and must be resolved by the backend.
	Index int64
	// HasIndex distinguishes an addressed element from the whole document.
	HasIndex bool

	// Value is what to write, for JSONSet and JSONAppend.
	Value protoreflect.Value
	// HasValue reports whether Value is meaningful.
	HasValue bool

	// At is where in the document this came from.
	At patch.At
}

// TestWant is what a [Test] asserts.
type TestWant int

const (
	// TestEqual asserts the address holds exactly this value.
	TestEqual TestWant = iota
	// TestExists asserts the address is present.
	TestExists
	// TestAbsent asserts the address is not present.
	TestAbsent
)

func (w TestWant) String() string {
	switch w {
	case TestEqual:
		return "equal"
	case TestExists:
		return "exists"
	case TestAbsent:
		return "absent"
	}
	return fmt.Sprintf("TestWant(%d)", int(w))
}

// Test is a predicate the row must satisfy for the plan to apply.
//
// A backend issues these as the UPDATE's WHERE clause, next to the predicate
// that identifies the row. A test that does not hold therefore matches no row,
// and the whole plan does not apply -- which is the atomicity the format
// requires, obtained from the statement rather than from a copy.
type Test struct {
	// Prop is the column under test. For a [graph.Edge] the comparison is
	// against the foreign key.
	Prop graph.Prop

	// Key addresses one entry of a map column; HasKey distinguishes it from the
	// column as a whole.
	Key    protoreflect.MapKey
	HasKey bool

	// Index addresses one element of a list column.
	Index    int64
	HasIndex bool

	Want TestWant

	// Value is the expected value, for TestEqual.
	Value protoreflect.Value

	// At is where in the document this came from.
	At patch.At
}
