package graph

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/protobuf-orm/protobuf-orm/internal/iters"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Index interface {
	// Entity which holds this index.
	Entity() Entity

	Name() string
	Number() protoreflect.FieldNumber
	Props() iter.Seq[Prop]

	IsUnique() bool
	IsImmutable() bool
	IsHidden() bool

	// ExcludesErased reports whether this index covers only the rows that are
	// still there, which a backend writes as a partial index.
	//
	// It is true of a unique index on an entity that erases softly, unless that
	// index says `includes_erased`. The default is the point of it: a row that
	// is gone should not go on holding the name it had, or the alias of
	// something erased could never be used again -- and a schema author who has
	// to remember to ask for that is one who finds out from a caller.
	ExcludesErased() bool
}

type protoIndex struct {
	// Entity which this index is applied on.
	entity *protoEntity
	opts   *ormpb.Index

	props []Prop
}

func parseIndex(
	ctx context.Context,
	e *protoEntity,
	opts *ormpb.Index,
) (*protoIndex, error) {
	v := &protoIndex{
		entity: e,
		opts:   opts,
		props:  []Prop{},
	}

	if len(opts.GetRefs()) == 0 {
		return nil, errors.New(": index must reference at least one prop")
	}

	errs := []error{}
	for i, ref := range opts.GetRefs() {
		prop, ok := iters.Find(e.Props(), func(p Prop) bool {
			return int32(p.Number()) == ref.GetNumber()
		})
		if !ok {
			errs = append(errs, fmt.Errorf("[%d(%s:%d)]: reference not found", i, ref.GetName(), ref.GetNumber()))
			continue
		}
		if name := string(prop.FullName().Name()); name != ref.GetName() {
			errs = append(errs, fmt.Errorf("[%d(%s:%d)]: name not matched, expected %q but referenced prop is named %q", i, ref.GetName(), ref.GetNumber(), ref.GetName(), name))
			continue
		}

		v.props = append(v.props, prop)
	}

	// Refused rather than ignored, on both counts. `includes_erased` says
	// something only about a unique index of an entity that erases softly, and
	// a declaration that says nothing is a declaration somebody wrote for a
	// reason that is not going to happen.
	if opts.GetIncludesErased() {
		if !opts.GetUnique() {
			errs = append(errs, errors.New(": includes_erased says nothing about an index that is not unique"))
		} else if !e.HasErasedField() {
			errs = append(errs, errors.New(": includes_erased says nothing about an entity that has no erased field"))
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return v, nil
}

func (i *protoIndex) Entity() Entity {
	return i.entity
}

func (i *protoIndex) Name() string {
	return i.opts.GetName()
}

func (i *protoIndex) Number() protoreflect.FieldNumber {
	return i.props[0].Number()
}

func (i *protoIndex) Props() iter.Seq[Prop] {
	return slices.Values(i.props)
}

func (i *protoIndex) IsUnique() bool {
	return i.opts.GetUnique()
}

func (i *protoIndex) IsImmutable() bool {
	return i.opts.GetImmutable()
}

func (i *protoIndex) IsHidden() bool {
	return i.opts.GetHidden()
}

func (i *protoIndex) ExcludesErased() bool {
	// Only uniqueness is a problem an erased row can cause. A non-unique index
	// that goes on covering erased rows costs a little space and answers a
	// query somebody may well want -- what was there before -- so it is left
	// alone.
	if !i.IsUnique() || i.opts.GetIncludesErased() {
		return false
	}

	return i.entity.HasErasedField()
}
