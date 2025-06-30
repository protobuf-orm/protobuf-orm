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
	Fields() iter.Seq[Field]
	Edges() iter.Seq[Edge]
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

	// Find inverse for the edges.
	for edge_ := range v.Edges() {
		edge := edge_.(*protoEdge)
		if !edge.opts.HasFrom() {
			continue
		}

		from := edge.opts.GetFrom()
		if err := (func() error {
			prop, ok := find(edge.target.Props(), func(v Prop) bool {
				return v.Number() == protoreflect.FieldNumber(from.GetNumber())
			})
			if !ok {
				return fmt.Errorf("back reference not found on the target entity: %s[%d]", edge.target.FullName(), from.GetNumber())
			}
			if name := prop.FullName().Name(); name != protoreflect.Name(from.GetName()) {
				return fmt.Errorf("name of back reference different from the one specified: %q!=%q", from.GetName(), name)
			}

			inverse, ok := prop.(Edge)
			if !ok {
				return fmt.Errorf("back reference is not an edge: %s", prop.FullName())
			}

			edge.inverse = inverse
			return nil
		})(); err != nil {
			errs = append(errs, fmt.Errorf(".%s: %w", edge.FullName().Name(), err))
		}
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

func (e *protoEntity) Fields() iter.Seq[Field] {
	return func(yield func(Field) bool) {
		for p := range e.Props() {
			if v, ok := p.(Field); ok {
				if !yield(v) {
					break
				}
			}
		}
	}
}

func (e *protoEntity) Edges() iter.Seq[Edge] {
	return func(yield func(Edge) bool) {
		for p := range e.Props() {
			if v, ok := p.(Edge); ok {
				if !yield(v) {
					break
				}
			}
		}
	}
}
