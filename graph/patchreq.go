package graph

import (
	"fmt"
	"iter"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// The layout of a generated PatchRequest.
//
// A Patch RPC spends one request field on each mutable prop and lets presence
// carry the intent: a field the caller set is a column the caller wants
// written. Nothing stores that shape -- it is derived from the entity, here, so
// that the generator that emits the message and the code that reads one back
// agree by construction rather than by comment.
//
// For a prop with entity field number N:
//
//	2N-1  the value
//	2N    its companion bool, when the prop has one
//
// and `ref` sits at the key's own number. Immutable props are absent
// altogether, which is what keeps `ref` off a prop's slot -- as long as the key
// is number 1. [PatchLayout] is where that "as long as" is checked.

const (
	// firstReservedNumber..lastReservedNumber is the band protobuf reserves
	// for its own use. Doubling a prop number can land in it.
	firstReservedNumber protoreflect.FieldNumber = 19000
	lastReservedNumber  protoreflect.FieldNumber = 19999

	// maxFieldNumber is protobuf's hard cap, 2^29-1.
	maxFieldNumber protoreflect.FieldNumber = 536870911
)

// PatchRefNumber is the number of the PatchRequest's `ref` field.
//
// It borrows the key's number, which is free only because the key is immutable
// and therefore never claims a slot of its own.
func PatchRefNumber(e Entity) protoreflect.FieldNumber {
	return e.Key().Number()
}

// PatchProps yields, in declaration order, the props a PatchRequest carries.
//
// An immutable prop is omitted rather than rejected later: the request has no
// field shape for something that cannot be written.
func PatchProps(e Entity) iter.Seq[Prop] {
	return func(yield func(Prop) bool) {
		for p := range e.Props() {
			if p.IsImmutable() {
				continue
			}
			if !yield(p) {
				return
			}
		}
	}
}

// PatchValueNumber is the number p's value occupies in the PatchRequest.
func PatchValueNumber(p Prop) protoreflect.FieldNumber {
	return p.Number()*2 - 1
}

// PatchFlagNumber is the number of p's companion bool.
//
// It is meaningful only when [PatchFlagOf] is not [PatchFlagNone]; for a prop
// with no companion the slot is simply not used.
func PatchFlagNumber(p Prop) protoreflect.FieldNumber {
	return p.Number() * 2
}

// PatchFlag is the companion bool a prop carries in a PatchRequest, if any.
//
// The two are exclusive by construction: a version field is validated to be
// non-nullable, so no prop can want both.
type PatchFlag int

const (
	// PatchFlagNone is a prop with no companion.
	PatchFlagNone PatchFlag = iota

	// PatchFlagForce is the version field's `_force`: write the version the
	// request carries instead of asserting it.
	PatchFlagForce

	// PatchFlagNull is a nullable prop's `_null`: clear the column. It has to
	// be a separate field because the format has no null -- an unset value
	// field means "leave it alone", not "set it to nothing".
	PatchFlagNull
)

// PatchFlagOf reports which companion p carries.
func PatchFlagOf(p Prop) PatchFlag {
	if f, ok := p.(Field); ok && f.IsVersion() {
		return PatchFlagForce
	}
	if p.IsNullable() {
		return PatchFlagNull
	}
	return PatchFlagNone
}

// PatchFlagName is the name of p's companion bool, or "" when it has none.
func PatchFlagName(p Prop) string {
	switch PatchFlagOf(p) {
	case PatchFlagForce:
		return p.Name() + "_force"
	case PatchFlagNull:
		return p.Name() + "_null"
	default:
		return ""
	}
}

// PatchSlotKind says what a PatchRequest field number means.
type PatchSlotKind int

const (
	// PatchSlotRef is the `ref` field.
	PatchSlotRef PatchSlotKind = iota + 1
	// PatchSlotValue is a prop's value.
	PatchSlotValue
	// PatchSlotFlag is a prop's companion bool.
	PatchSlotFlag
)

// PatchSlot is one field of a PatchRequest, resolved back onto the entity.
type PatchSlot struct {
	Kind PatchSlotKind
	// Prop is the prop the slot belongs to, nil when Kind is PatchSlotRef.
	Prop Prop
}

// PatchLayout resolves every field of e's PatchRequest and reports the mapping,
// or the first reason the entity cannot have one.
//
// The arithmetic is injective across props -- 2M-1 and 2N-1 differ for M != N,
// and an odd slot never meets an even one -- so a prop can only collide with
// `ref`, which sits at the key's number K. That happens for every K but 1: an
// odd K equals the value slot of the prop numbered (K+1)/2, and an even K
// equals the companion slot of the prop numbered K/2. Nothing in this package
// requires K == 1, so the collision is checked rather than assumed.
//
// Doubling also has a ceiling: a prop numbered in [9500, 10000] lands in
// protobuf's reserved band, and one above 268435456 overflows the cap.
func PatchLayout(e Entity) (map[protoreflect.FieldNumber]PatchSlot, error) {
	out := map[protoreflect.FieldNumber]PatchSlot{}

	describe := func(s PatchSlot) string {
		switch s.Kind {
		case PatchSlotRef:
			return "ref"
		case PatchSlotFlag:
			return PatchFlagName(s.Prop)
		default:
			return s.Prop.Name()
		}
	}
	put := func(n protoreflect.FieldNumber, s PatchSlot) error {
		if n < 1 || n > maxFieldNumber {
			return fmt.Errorf(
				"%s: %s would take field number %d, which protobuf does not allow",
				e.FullName(), describe(s), n)
		}
		if n >= firstReservedNumber && n <= lastReservedNumber {
			return fmt.Errorf(
				"%s: %s would take field number %d, which falls in protobuf's "+
					"reserved range %d-%d; number the prop outside [%d, %d]",
				e.FullName(), describe(s), n,
				firstReservedNumber, lastReservedNumber,
				firstReservedNumber/2, (lastReservedNumber+1)/2)
		}
		if prev, ok := out[n]; ok {
			return fmt.Errorf(
				"%s: %s and %s would both take field number %d; the request puts "+
					"ref at the key's number, so the key must be number 1",
				e.FullName(), describe(prev), describe(s), n)
		}
		out[n] = s
		return nil
	}

	if err := put(PatchRefNumber(e), PatchSlot{Kind: PatchSlotRef}); err != nil {
		return nil, err
	}
	for p := range PatchProps(e) {
		if err := put(PatchValueNumber(p), PatchSlot{Kind: PatchSlotValue, Prop: p}); err != nil {
			return nil, err
		}
		if PatchFlagOf(p) == PatchFlagNone {
			continue
		}
		if err := put(PatchFlagNumber(p), PatchSlot{Kind: PatchSlotFlag, Prop: p}); err != nil {
			return nil, err
		}
	}

	return out, nil
}
