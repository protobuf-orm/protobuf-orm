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
	Path() string
	Package() string

	FullName() protoreflect.FullName
	Rpcs() RpcMap
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

	rpcs *rpcMap

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

	errs := []error{}

	// Parse props.
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
	for field := range v.Fields() {
		f := field.(*protoField)
		if !f.opts.GetKey() {
			continue
		}
		if v.key != nil {
			return nil, fmt.Errorf(": there can be only one key, found %s:%d and %s:%d",
				v.key.FullName().Name(), v.key.Number(),
				f.FullName().Name(), f.Number(),
			)
		}
		if f.opts.HasUnique() && !f.opts.GetUnique() {
			return nil, fmt.Errorf(".%s: key must be unique", f.FullName().Name())
		}

		v.key = f
		f.opts.SetUnique(true)
	}
	if v.key == nil {
		return nil, fmt.Errorf(": no key is defined")
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

	v.rpcs = parseRpcs(ctx, g, v, opts.GetRpc())

	return v, nil
}

func (e *protoEntity) Path() string {
	return e.source.ParentFile().Path()
}

func (e protoEntity) Package() string {
	return string(e.source.ParentFile().Package())
}

func (e *protoEntity) FullName() protoreflect.FullName {
	return e.source.FullName()
}

func (e *protoEntity) Rpcs() RpcMap {
	return e.rpcs
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
