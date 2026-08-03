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
//	2N    the value
//	2N+1  its companion bool, when the prop has one
//
// and `ref` is always number 1. Prop numbers start at 1, so the lowest slot any
// prop can claim is 2 and slot 1 is free in every schema -- which is the whole
// reason the band starts where it does.
//
// The obvious alternative, giving `ref` the key's own number, does not work.
// Under a 2N-1 / 2N layout the props claim every number from 1 up, so there is
// no slot `ref` could take; it is safe at the key's number only when the prop
// that would have claimed that slot IS the key, and the key, being immutable,
// is absent from the request. That holds at number 1 and nowhere else, which
// made an arithmetic accident into a schema constraint. Shifting the band by
// one removes the constraint instead of enforcing it, and incidentally puts
// `ref` where every other request in the family already has it.

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
// It takes an entity only so that callers read the layout from one place; the
// answer is 1 for every entity, because no prop can reach that low.
func PatchRefNumber(e Entity) protoreflect.FieldNumber {
	return 1
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
	return p.Number() * 2
}

// PatchFlagNumber is the number of p's companion bool.
//
// It is meaningful only when [PatchFlagOf] is not [PatchFlagNone]; for a prop
// with no companion the slot is simply not used.
func PatchFlagNumber(p Prop) protoreflect.FieldNumber {
	return p.Number()*2 + 1
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
// Slots cannot collide: 2M and 2N differ for M != N, an even slot never meets
// an odd one, and `ref` sits below every slot a prop can reach. The duplicate
// check stays anyway, because it is the invariant the rest of this file rests
// on and a future slot would be caught by it rather than by a confused parser.
//
// What can still go wrong is arithmetic. Doubling a prop number in
// [9500, 9999] lands in protobuf's reserved band, and one above 268435455
// overflows the cap -- both are the entity's to fix and neither is obvious from
// the number the author wrote.
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
				firstReservedNumber/2, lastReservedNumber/2)
		}
		if prev, ok := out[n]; ok {
			return fmt.Errorf(
				"%s: %s and %s would both take field number %d, which the layout "+
					"is supposed to make impossible",
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
