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

// Entity is a schema description.
type Entity interface {
	FullName() protoreflect.FullName
	Key() Field
	Props() iter.Seq[Prop]
	Fields() iter.Seq[Field]
	Edges() iter.Seq[Edge]
	Indexes() iter.Seq[Index]
}

// Entity parsed from the proto message.
type protoEntity struct {
	// Proto message which this entity is based on.
	source protoreflect.MessageDescriptor

	// Proto field which represents a key.
	key     *protoField
	props   []Prop
	indexes []Index
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

	// Forward declaration for
	// - self-reference
	// - circular reference
	g.Entities[m.FullName()] = v

	// TODO: fill rpcs and indexes

	errs := []error{}

	// Paras props.
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

	// Parse indexes.
	for i, index_opt := range opts.GetIndexes() {
		index, err := parseIndex(ctx, v, index_opt)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%d(%s)]%w", i, index_opt.GetName(), err))
		}

		v.indexes = append(v.indexes, index)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf(".{indexes}%w", errors.Join(errs...))
	}

	// Find inverse for the edges.
	for edge_ := range v.Edges() {
		edge := edge_.(*protoEdge)
		if !edge.opts.HasFrom() {
			continue
		}

		back_ref := edge.opts.GetFrom()
		if err := (func() error {
			prop, ok := iters.Find(edge.target.Props(), func(v Prop) bool {
				return v.Number() == protoreflect.FieldNumber(back_ref.GetNumber())
			})
			if !ok {
				return fmt.Errorf("back reference not found on the target entity: %s[%d]", edge.target.FullName(), back_ref.GetNumber())
			}
			if name := string(prop.FullName().Name()); name != back_ref.GetName() {
				return fmt.Errorf("name of back reference different from the one specified: %q!=%q", back_ref.GetName(), name)
			}

			inverse, ok := prop.(*protoEdge)
			if !ok {
				return fmt.Errorf("back reference is not an edge: %s", prop.FullName())
			}

			if inverse.IsUnique() && edge.source.Cardinality() == protoreflect.Repeated {
				// Back reference is marked as unique but target has repeated cardinality.
				return fmt.Errorf("back reference is unique edge so it cannot have repeated cardinality")
			}
			if inverse.source.Cardinality() != protoreflect.Repeated && edge.source.Cardinality() != protoreflect.Repeated {
				// O2O relation.
				inverse.opts.SetUnique(true)
				edge.opts.SetUnique(true)
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

func (e *protoEntity) Indexes() iter.Seq[Index] {
	return slices.Values(e.indexes)
}
