package graph

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
)

type Index interface {
	Name() string
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

	errs := []error{}
	for i, ref := range opts.GetRefs() {
		ok := false
		for prop := range e.Props() {
			number := int32(prop.Number())
			if number != ref.GetNumber() {
				continue
			}
			if name := string(prop.FullName().Name()); name != ref.GetName() {
				errs = append(errs, fmt.Errorf("[%d(%s:%d)]: name not matched, ref name was %s", i, ref.GetName(), ref.GetNumber(), name))
				continue
			}

			v.props = append(v.props, prop)

			ok = true
			break
		}
		if ok {
			continue
		}

		errs = append(errs, fmt.Errorf("[%d(%s:%d)]: reference not found", i, ref.GetName(), ref.GetNumber()))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return v, nil
}

func (i *protoIndex) Name() string {
	return i.opts.GetName()
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
