package ormpatch

import (
	"errors"
	"fmt"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// EdgeResolver turns the Ref a PatchRequest carries for an edge into the value
// of the target entity's key.
//
// It has to be supplied rather than derived because a document can never carry
// a Ref: [Compile] reads no storage, and a Ref may address its target by a
// unique field that is not the key. Turning that into a key is a query, so it
// stays outside the conversion -- and outside this package, which has no
// database.
//
// The returned value must be the key's own proto kind: raw bytes for a UUID
// key, a string for a string key. It is encoded against the target key's
// descriptor, so anything else is refused rather than coerced.
type EdgeResolver func(ed graph.Edge, ref protoreflect.Message) (protoreflect.Value, error)

// ErrRequestLayout marks a request whose fields are not where the entity says
// they should be.
//
// It is not a client's fault and must not be reported as one: the request
// message and the entity come from the same generator, so a mismatch means the
// two were generated from different schemas.
var ErrRequestLayout = errors.New("ormpatch: request layout does not match the entity")

// VersionNotGivenError is a request that neither carries the version nor asks
// to force past it.
//
// The format cannot express this refusal -- a missing precondition is simply a
// document without a test -- so it has to be caught here. Letting it through
// would turn optimistic locking into an opt-in that any caller can forget.
type VersionNotGivenError struct {
	Prop graph.Field
}

func (e *VersionNotGivenError) Error() string {
	return fmt.Sprintf("version not given: %s", e.Prop.Name())
}

// FromPatchRequest converts a generated XxxPatchRequest into a patch document
// addressed against e.
//
// The request's layout is derived, not looked up: see [graph.PatchLayout]. The
// `ref` field is deliberately not read -- it selects the row, which is the
// caller's predicate, not part of the delta.
//
// It returns (nil, nil) when the request asks for nothing. That is not the same
// as an empty document: a Delta must carry at least one entry, so "change
// nothing" has to be the absence of a document rather than an empty one, and
// the caller decides what an absent document means for it.
//
// Three prop classes deviate from "presence means assign", and each deviation
// is a preserved behavior rather than a choice made here:
//
//   - A collection has no presence in the request, so only a non-empty one is
//     converted. A PatchRequest therefore cannot clear a map or a list; a
//     hand-written document can.
//   - A nullable prop whose `_null` companion is set becomes `remove`, and the
//     companion wins outright. Emitting both a remove and an assign would fold
//     into the assign silently.
//   - The version field becomes a test, or an assign when `_force` carries a
//     value, or nothing at all when `_force` carries none. It is never stamped
//     here: a document cannot say now(), and the server that owns the clock is
//     the one that should.
func FromPatchRequest(e graph.Entity, req protoreflect.Message, resolve EdgeResolver) (*patchpb.Patch, error) {
	if e == nil {
		return nil, patch.Errf(patch.CodeMissingField, "", "no entity")
	}
	if req == nil {
		return nil, patch.Errf(patch.CodeMissingField, "", "no request")
	}

	rd := req.Descriptor()

	var entries []*patchpb.Entry
	for p := range graph.PatchProps(e) {
		vfd := rd.Fields().ByNumber(graph.PatchValueNumber(p))
		if vfd == nil {
			return nil, fmt.Errorf("%w: %s declares no field %d, where %s.%s belongs",
				ErrRequestLayout, rd.FullName(), graph.PatchValueNumber(p), e.FullName(), p.Name())
		}
		at := patch.At("req." + string(vfd.Name()))

		flag := false
		if graph.PatchFlagOf(p) != graph.PatchFlagNone {
			ffd := rd.Fields().ByNumber(graph.PatchFlagNumber(p))
			if ffd == nil || ffd.Kind() != protoreflect.BoolKind || ffd.IsList() || ffd.IsMap() {
				return nil, fmt.Errorf("%w: %s declares no bool %q at %d",
					ErrRequestLayout, rd.FullName(), graph.PatchFlagName(p), graph.PatchFlagNumber(p))
			}
			flag = req.Get(ffd).Bool()
		}

		set := requestHas(req, vfd)

		switch graph.PatchFlagOf(p) {
		case graph.PatchFlagForce:
			f := p.(graph.Field)
			switch {
			case !flag && !set:
				return nil, &VersionNotGivenError{Prop: f}

			case !flag:
				// The version is a precondition. It is a test, so the update
				// stays one compare-and-swap statement.
				v, err := requestValue(req, vfd, p.Descriptor(), at)
				if err != nil {
					return nil, err
				}
				entries = append(entries, entryTest(p, v))

			case set:
				// Forced AND carrying a value used to mean "store this
				// version", which handed the client the token everyone else's
				// compare-and-swap is measured against. See [VersionWriteError]
				// for why that could not stay. Refusing rather than ignoring
				// the value keeps a caller who meant one of the other three
				// cells from silently getting a fourth.
				return nil, &VersionWriteError{At: at, Prop: f.Name()}

				// case flag && !set: neither asserted nor written. The server
				// stamps the column because the document does not.
			}
			continue

		case graph.PatchFlagNull:
			if flag {
				entries = append(entries, entryRemove(p))
				continue
			}
		}

		if !set {
			continue
		}

		switch p := p.(type) {
		case graph.Edge:
			if p.IsList() {
				return nil, unsupportedf(at,
					"%s is a repeated edge; the row holds no column for it", p.FullName())
			}
			if resolve == nil {
				return nil, fmt.Errorf("ormpatch: %s is set but no EdgeResolver was given", p.FullName())
			}

			kv, err := resolve(p, req.Get(vfd).Message())
			if err != nil {
				// Returned as it came: a resolver's error is usually a lookup's,
				// and wrapping it would bury the code it carries.
				return nil, err
			}

			key := p.Target().Key()
			kfd := key.Descriptor()
			v, err := valueOf(guardBytes(kv, kfd), kfd, at)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entryEdgeAssign(p, key, v))

		case graph.Field:
			v, err := requestValue(req, vfd, p.Descriptor(), at)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entryAssign(p, v))

		default:
			return nil, fmt.Errorf("ormpatch: %s: unknown prop type %T", p.FullName(), p)
		}
	}

	if len(entries) == 0 {
		return nil, nil
	}
	return patchpb.Patch_builder{
		MessageType: proto.String(string(e.FullName())),
		Delta:       patchpb.Delta_builder{Entries: entries}.Build(),
	}.Build(), nil
}

// requestHas reports whether the request asks about this field at all.
//
// A repeated or map field in a PatchRequest has no presence -- edition 2023
// gives explicit presence to singular fields only -- so emptiness is the only
// signal there is.
func requestHas(req protoreflect.Message, fd protoreflect.FieldDescriptor) bool {
	switch {
	case fd.IsMap():
		return req.Get(fd).Map().Len() > 0
	case fd.IsList():
		return req.Get(fd).List().Len() > 0
	default:
		return req.Has(fd)
	}
}

// requestValue reads a request field and encodes it against the ENTITY's
// descriptor.
//
// The entity's descriptor is the one that decides the arm. [patch.ValueOf]
// resolves the site itself, so handing it the request's descriptor -- or a
// descriptor already resolved to a map value or a list element -- would resolve
// twice and encode a map as a message.
func requestValue(
	req protoreflect.Message,
	vfd protoreflect.FieldDescriptor,
	entfd protoreflect.FieldDescriptor,
	at patch.At,
) (*patchpb.Value, error) {
	return valueOf(guardBytes(req.Get(vfd), entfd), entfd, at)
}

func valueOf(pv protoreflect.Value, fd protoreflect.FieldDescriptor, at patch.At) (*patchpb.Value, error) {
	v, err := patch.ValueOf(pv, fd, patch.SiteField, at)
	if err != nil {
		return nil, err
	}
	// ValueOf encodes what the message holds without judging it, so it can
	// return a value its own CheckArm refuses. The reachable case is a closed
	// enum carrying a number the enum does not declare, since the generated Go
	// type is an int32 that accepts anything. Checking here fails at the
	// request, where the field has a name, instead of at Compile, where it
	// reads as a malformed document.
	if err := patch.CheckArm(v, fd, patch.SiteField, at); err != nil {
		return nil, err
	}
	return v, nil
}

// guardBytes normalizes a nil []byte.
//
// [patch.ValueOf] does this itself now. It is kept because nothing pins that
// version: built against an older protobuf-patch, a nil slice leaves the arm
// unset and the value says nothing at all. It costs one comparison.
func guardBytes(pv protoreflect.Value, fd protoreflect.FieldDescriptor) protoreflect.Value {
	if fd.IsList() || fd.IsMap() || fd.Kind() != protoreflect.BytesKind {
		return pv
	}
	if pv.Bytes() != nil {
		return pv
	}
	return protoreflect.ValueOfBytes([]byte{})
}

// propKey pins both the name and the number.
//
// They combine as constraints rather than fallbacks, so a document built
// against a renumbered entity is a loud CodeFieldConflict instead of a silent
// write to whichever column now holds that number.
func propKey(p graph.Prop) *patchpb.Key {
	return patchpb.Key_builder{Field: patchpb.Field_builder{
		Name:   proto.String(p.Name()),
		Number: proto.Uint32(uint32(p.Number())),
	}.Build()}.Build()
}

func oneTarget(k *patchpb.Key) *patchpb.Targets {
	return patchpb.Targets_builder{Selectors: []*patchpb.Selector{
		patchpb.Selector_builder{Key: k}.Build(),
	}}.Build()
}

func entryAssign(p graph.Prop, v *patchpb.Value) *patchpb.Entry {
	return patchpb.Entry_builder{
		Targets: oneTarget(propKey(p)),
		Assign:  patchpb.Assign_builder{Value: v}.Build(),
	}.Build()
}

// entryRemove clears a column. There is no null in the format: an assign of an
// empty value writes the empty value, which is a different row.
func entryRemove(p graph.Prop) *patchpb.Entry {
	return patchpb.Entry_builder{
		Targets: oneTarget(propKey(p)),
		Remove:  &patchpb.Remove{},
	}.Build()
}

// entryTest carries no on_missing and no on_absent_path: a test must be strict,
// and a document that softens one is rejected outright.
func entryTest(p graph.Prop, v *patchpb.Value) *patchpb.Entry {
	return patchpb.Entry_builder{
		Targets: oneTarget(propKey(p)),
		Test:    patchpb.Test_builder{Value: v}.Build(),
	}.Build()
}

// entryEdgeAssign addresses the edge through its path and the target's key as
// the selector. Assigning the edge field itself is refused: the value would
// have to be a whole target message where the row holds only its key.
func entryEdgeAssign(ed graph.Edge, key graph.Field, v *patchpb.Value) *patchpb.Entry {
	return patchpb.Entry_builder{
		Path:    patchpb.Path_builder{Segments: []*patchpb.Key{propKey(ed)}}.Build(),
		Targets: oneTarget(propKey(key)),
		Assign:  patchpb.Assign_builder{Value: v}.Build(),
	}.Build()
}
