package graph

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Entity is a schema description.
type Entity interface {
	FullName() protoreflect.FullName
	Key() Field
	Props() iter.Seq[Prop]
}

// Entity parsed from the proto message.
type protoEntity struct {
	// Proto message which this entity is based on.
	source protoreflect.MessageDescriptor

	// Proto field which represents a key.
	key   *protoField
	props []Prop
}

func parseEntity(
	ctx context.Context,
	g *Graph,
	m protoreflect.MessageDescriptor,
	opts *ormpb.MessageOptions,
) (*protoEntity, error) {
	v := &protoEntity{
		source: m,
	}

	// Prevents errors from
	// - self-reference
	// - circular reference
	g.Entities[m.FullName()] = v

	// TODO: fill rpcs and indexes

	errs := []error{}
	for i := 0; i < m.Fields().Len(); i++ {
		mf := m.Fields().Get(i)
		prop, err := parseProp(ctx, g, v, mf)
		if err != nil {
			errs = append(errs, fmt.Errorf(".%s: %w", mf.Name(), err))
			continue
		}
		if prop == nil {
			// Disabled prop.
			continue
		}

		v.props = append(v.props, prop)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return v, nil
}

func (e *protoEntity) FullName() protoreflect.FullName {
	return e.source.FullName()
}

func (e *protoEntity) Key() Field {
	return e.key
}

func (e *protoEntity) Props() iter.Seq[Prop] {
	return slices.Values(e.props)
}
