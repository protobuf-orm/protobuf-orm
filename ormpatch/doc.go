// Package ormpatch compiles a patch document into the writes and predicates a
// single row UPDATE needs.
//
// It is a second engine for the protobuf-patch format. The reference engine,
// patchproto, applies a document to a message in memory; this one never
// materializes the entity. It resolves the document against a [graph.Entity]
// and returns a [Plan] — a set of column writes and a set of predicates — which
// a storage backend turns into one statement.
//
// That difference is the point. A read-modify-write loses a concurrent update
// between the read and the write; compiling to one statement does not. A `test`
// entry becomes a WHERE predicate, so optimistic locking is the document's own
// business rather than a protocol field, and it stays a compare-and-swap.
//
// # What a row can hold
//
// The addressable surface of an entity row is smaller than a message's, and
// every rule below follows from that. A row has scalar columns, foreign-key
// columns for edges, and JSON columns for maps and lists. It has no nested
// message: a [graph.Field] is never TYPE_MESSAGE, only TYPE_JSON or TYPE_TIME,
// each of which is one column.
//
// So exactly these addresses compile, and everything else is refused:
//
//	Field(f)                          a scalar, time, uuid, enum or json column
//	Field(e)                          an edge, for remove and test only
//	Field(k) under path Field(e)      the edge's foreign key -- see below
//	MapKey(k) under path Field(f)     one entry of a map column
//	Index(i) under path Field(f)      one element of a list column
//	Append() under path Field(f)      the end of a list column
//	EveryEntry() under path Field(f)  every entry of a map column
//	container under path Field(f)     a map or list column as a whole
//
// # Addressing an edge
//
// An edge is declared as the target message — `User parent = 10` — so the
// format would have a document carry a whole `User` to repoint it. That is the
// wrong shape twice over: the value would be a message where the row holds a
// key, and `test` compares messages exactly, so asserting the current target
// would also assert that every other field of it is unset, which no stored row
// satisfies.
//
// Instead an edge is addressed through it, at the target entity's key:
//
//	patch.Target(patch.Name("id")).In(patch.Name("parent")).Assign(patch.Bytes(k))
//
// The value is then the key's own type, `test` is a scalar comparison, and both
// compile to the foreign-key column directly. Nothing about this is special
// pleading: it is an ordinary path into a message field, and the reference
// engine applies it to a snapshot with the same meaning. That matters for an
// audit log that stores deltas and replays them.
//
// Clearing a nullable edge is [patchpb.Remove] on the edge field itself, which
// nulls the foreign key.
//
// # Order, and why entries are folded
//
// The format says entries apply in order and each observes the effects of those
// before it. A single UPDATE evaluates every assignment against the row as it
// was, so the two agree only while no entry reads a column an earlier entry
// wrote. Compile detects that case and refuses it rather than emitting a
// statement with a different meaning -- see [ErrUnsupported].
//
// Writes to one column are folded, last-wins, which is what re-applying them in
// order would produce anyway.
//
// # Positions in a list
//
// The same disagreement takes a sharper form for list indices, because an index
// is not a name. Removing an element moves everything after it, so an index
// means a position in the list as the entries before it left it -- while the
// guard that stops an out-of-range write from landing somewhere else is a
// predicate, and a predicate reads the row as it was.
//
// The edits themselves are fine: they nest inside one assignment and apply in
// order, which is what the format says. It is only the guard that asks about
// the wrong state. So the rule is not one index per document:
//
// Most of it is not a real limit, because the rewrite is mechanical:
// remove-then-assign says the same thing as assign-then-remove with the index
// adjusted. [patch.Normalize] does that, and Compile runs it first -- so a
// document written in the order its author thought in compiles, and what is
// left refused is what normalization could not prove.
//
//	elem(0).Assign(); elem(2).Assign()     fine, nothing moved
//	elem(1).Assign(); elem(0).Remove()     fine, the remove is last
//	elem(0).Remove(); elem(2).Assign()     reordered, then fine
//	append();         elem(2).Assign()     refused: an append lands at the old
//	                                       length, so the one index it moves is
//	                                       the one only the row knows
//
// A negative index is refused for the same reason in its purest form: it counts
// from the end, and the length is in the row this statement is writing.
//
// # Comparing a collection
//
// A map or a list is one JSON document in one column. Comparing an entry is a
// real question and the database answers it. Comparing the WHOLE column against
// a literal is not: it asks whether two serializations match, and entry order,
// key order and the spelling of each value all decide that. Two collections
// that are equal in every sense the format cares about can compare unequal, and
// a partial edit changes the spelling for good.
//
// So it is refused. Test an entry, or lock on a version field. Assigning a
// whole collection is untouched -- writing a serialization is what the column
// is for.
//
// # Divergence from the reference engine
//
// Agreement with patchproto is defined by [patch.CodeOf] and by the resulting
// value. Where this engine cannot honor a document it refuses it with
// [ErrUnsupported], which carries no [patch.Code] — the document is valid and
// the reference engine would apply it; this engine declines. Callers should map
// a [patch.Code] to InvalidArgument and an [ErrUnsupported] to Unimplemented,
// so the two stay distinguishable to a client.
//
// [DeclaredDivergences] lists every case, with its reason.
package ormpatch
