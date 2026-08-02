package ormpatch

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-patch/patch"
)

// ErrNoPatch is [Compile] handed no document at all.
//
// It is a package-level value rather than an anonymous error so that a server
// accepting a document in an optional field can report its absence in the same
// words the compiler would have used, without restating them.
var ErrNoPatch = patch.Errf(patch.CodeMissingField, "", "no patch")

// ErrUnsupported marks a document this engine declines to compile.
//
// It is deliberately NOT a [patch.Error]: nothing about the document is wrong,
// and the reference engine would apply it. A row simply cannot express what it
// asks — there is no nested message to descend into, no way to report that a
// map key was already absent, no way to read a column an earlier entry wrote.
//
// Keeping the two apart matters at the edge of a service. A [patch.Code] means
// the producer sent a bad document and should fix it, which is
// InvalidArgument. An ErrUnsupported means the document is fine and this
// backend is not, which is Unimplemented. Collapsing them would tell a client
// to correct something that is already correct.
//
// [patch.CodeOf] returns [patch.CodeOK] for these, so a handler that switches
// on the code will not mistake one for a format violation.
var ErrUnsupported = errors.New("ormpatch: unsupported by a row-backed engine")

// UnsupportedError is a refusal positioned in the document, wrapping
// [ErrUnsupported].
type UnsupportedError struct {
	At     patch.At
	Detail string
}

func (e *UnsupportedError) Error() string {
	if e.At == "" {
		return fmt.Sprintf("%v: %s", ErrUnsupported, e.Detail)
	}
	return fmt.Sprintf("%s: %v: %s", e.At, ErrUnsupported, e.Detail)
}

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

func unsupportedf(at patch.At, format string, args ...any) error {
	return &UnsupportedError{At: at, Detail: fmt.Sprintf(format, args...)}
}

// Divergence is one documented difference between this engine and the
// reference engine.
type Divergence struct {
	// What the document does.
	Construct string
	// Why a row cannot honor it.
	Cause string
}

// DeclaredDivergences is every construct this engine refuses that patchproto
// would apply, with the reason.
//
// It is a list rather than a comment so a test can hold it to the truth: a
// refusal that is not declared here, or a declaration this engine no longer
// needs, is a defect in one of the two.
var DeclaredDivergences = []Divergence{
	{
		Construct: "a target that is not a column",
		Cause: "the ORM does not model every declared field -- one carrying " +
			"orm.field.disabled parses to no prop and has no column, so it can " +
			"be addressed in the message but not written in the row",
	},
	{
		Construct: "assign or insert on an edge field",
		Cause: "the value would have to be the whole target message where the " +
			"row holds only its key; address the edge's key through it instead",
	},
	{
		Construct: "a path deeper than one segment, or into anything but an " +
			"edge, a map or a list",
		Cause: "a row has no nested message to descend into",
	},
	{
		Construct: "a path into an edge whose target is not the edge's key field",
		Cause: "an edge is one foreign-key column; nothing else about the " +
			"target entity is present in this row",
	},
	{
		Construct: "reading a column an earlier entry of the same document wrote " +
			"(a test after an assign, or a move/copy source)",
		Cause: "entries observe each other in the format, but one UPDATE " +
			"evaluates every assignment against the row as it was",
	},
	{
		Construct: "move and copy",
		Cause: "the source must be cleared after every target is written, which " +
			"is a read of a column the same statement assigns",
	},
	{
		Construct: "insert on a column that is not a list append",
		Cause: "insert must not overwrite, so it has to know whether the target " +
			"is occupied, which is a read",
	},
	{
		Construct: "nest into anything but a map, a list or an edge",
		Cause: "nesting materializes its target even when the inner delta only " +
			"asserts; in a row that would be an insert triggered by a read",
	},
	{
		Construct: "container scope at the root of the document",
		Cause: "it addresses the entity as a whole -- remove would clear every " +
			"column at once -- and no caller needs it through a patch; the row " +
			"is already identified by the request",
	},
	{
		Construct: "a container-scope test",
		Cause: "it asserts the container is non-empty, which is a different " +
			"question from any predicate on a column and is not worth a second " +
			"shape in the plan; test an entry instead",
	},
	{
		Construct: "a span of list elements",
		Cause: "the positions it names shift as the statement removes or " +
			"inserts, and one assignment cannot express the rebuild",
	},
	{
		Construct: "oneof_member",
		Cause: "which member is set is known only to the row",
	},
	{
		Construct: "a partial edit of a column the same document already wrote " +
			"whole",
		Cause: "one statement assigns a column once, and a modifier on a column " +
			"that a plain assignment also names silently discards the assignment",
	},
}

// A note on what is NOT a divergence, because the first design said it was.
//
// A missing target inside a JSON column -- removing a map key that is not
// there, writing past the end of a list -- looked unrepresentable, because an
// UPDATE reports how many rows it touched and never why. It compiles anyway:
// the requirement becomes a predicate, so an absent key matches no row and the
// whole document is abandoned. That is what a miss means, obtained from the
// statement rather than from a lookup.
//
// What is genuinely lost is only the error's precision. The caller sees that
// nothing applied, not which of the row, a test, or a missing target was
// responsible. A backend that wants to say which can read the row back once,
// after the fact, and pay for the answer only when it is needed.

// ImmutableError is a write to a prop the schema marks immutable.
//
// Like [ErrUnsupported] this is not a format violation -- the document is
// well-formed and the reference engine would apply it -- but unlike it, the
// refusal is the ORM's own rule rather than a limit of rows. The entity's key
// is always immutable, so this also covers a document that tries to re-key a
// row.
type ImmutableError struct {
	At   patch.At
	Prop string
}

func (e *ImmutableError) Error() string {
	return fmt.Sprintf("%s: %s is immutable", e.At, e.Prop)
}
