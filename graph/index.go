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
