package graph

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/protobuf-orm/protobuf-orm/internal/iters"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Entity is a schema description.
type Entity interface {
	Descriptor() protoreflect.MessageDescriptor
	Path() string
	Package() string

	FullName() protoreflect.FullName
	Name() string

	Rpcs() RpcMap
	Key() Field

	Elems() iter.Seq[Elem]
	Keys() iter.Seq[Elem]
	Props() iter.Seq[Prop]
	Fields() iter.Seq[Field]
	Edges() iter.Seq[Edge]
	Indexes() iter.Seq[Index]

	HasElems() bool
	HasProps() bool
	HasFields() bool
	HasEdges() bool
	HasIndexes() bool

	HasVersionField() bool
	GetVersionField() Field
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
) (*protoEntity, error) {
	if v, ok := g.Entities[m.FullName()]; ok {
		return v.(*protoEntity), nil
	}

	opts := proto.GetExtension(m.Options(), ormpb.E_Message).(*ormpb.MessageOptions)
	if opts == nil || opts.GetDisabled() {
		return nil, nil
	}

	v := &protoEntity{
		source: m,
	}

	// Forward declaration for
	// - self-reference
	// - circular reference
	g.Entities[m.FullName()] = v

	// The forward declaration above is only valid if this entity parses
	// successfully. Remove it on every failure path (including the direct
	// returns in the key/edge validation below) so a failed parse never leaves
	// a partially-built entity behind for other entities to reference.
	committed := false
	defer func() {
		if !committed {
			delete(g.Entities, m.FullName())
		}
	}()

	errs := []error{}

	// Normalizations (e.g. marking the key or an O2O edge unique) are collected
	// here and applied only once the whole entity has validated successfully.
	// Some of them mutate options that belong to *other*, already-parsed
	// entities (the target of an edge), so applying them eagerly would let a
	// later failure leave those shared entities in a half-updated state. This is
	// the commit half of a two-phase parse: stage on failure, apply on success.
	commits := []func(){}

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
			return nil, fmt.Errorf(": there can be only one key, found %s(%d) and %s(%d)",
				v.key.Name(), v.key.Number(),
				f.Name(), f.Number(),
			)
		}
		if f.opts.HasUnique() && !f.opts.GetUnique() {
			return nil, fmt.Errorf(".%s: key must be unique", f.Name())
		}
		if f.opts.HasNullable() && f.opts.GetNullable() {
			return nil, fmt.Errorf(".%s: key cannot be nullable", f.Name())
		}
		if f.opts.HasImmutable() && !f.opts.GetImmutable() {
			return nil, fmt.Errorf(".%s: key must be immutable", f.Name())
		}

		v.key = f
		commits = append(commits, func() {
			f.opts.SetUnique(true)
			f.opts.SetImmutable(true)
		})
	}
	if v.key == nil {
		return nil, fmt.Errorf(": no key is defined")
	}

	// At most one version field. [protoEntity.GetVersionField] answers with the
	// first, and a server stamps only the one it is given, so a second would be
	// a column frozen at whatever Add wrote while still demanding a value on
	// every patch -- a lock that always passes its own compare-and-swap.
	{
		var ver *protoField
		for field := range v.Fields() {
			f := field.(*protoField)
			if !f.IsVersion() {
				continue
			}
			if ver != nil {
				return nil, fmt.Errorf(": there can be only one version field, found %s(%d) and %s(%d)",
					ver.Name(), ver.Number(),
					f.Name(), f.Number(),
				)
			}
			ver = f
		}
	}

	// Parse indexes.
	for i, index_opt := range opts.GetIndexes() {
		index, err := parseIndex(ctx, v, index_opt)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%d(%s)]%w", i, index_opt.GetName(), err))
			continue
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
				commits = append(commits, func() {
					inverse.opts.SetUnique(true)
					edge.opts.SetUnique(true)
				})
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

	// Validation passed; apply the staged normalizations now (see commits above).
	for _, commit := range commits {
		commit()
	}

	committed = true
	return v, nil
}

func (e *protoEntity) Descriptor() protoreflect.MessageDescriptor {
	return e.source
}

func (e *protoEntity) Path() string {
	return e.source.ParentFile().Path()
}

func (e *protoEntity) Package() string {
	return string(e.source.ParentFile().Package())
}

func (e *protoEntity) FullName() protoreflect.FullName {
	return e.source.FullName()
}

func (e *protoEntity) Name() string {
	return string(e.source.FullName().Name())
}

func (e *protoEntity) Rpcs() RpcMap {
	return e.rpcs
}

func (e *protoEntity) Key() Field {
	return e.key
}

func (e *protoEntity) Elems() iter.Seq[Elem] {
	return func(yield func(Elem) bool) {
		for _, v := range e.props {
			if !yield(v) {
				return
			}
		}
		for _, v := range e.indexes {
			if !yield(v) {
				return
			}
		}
	}
}

func (e *protoEntity) Keys() iter.Seq[Elem] {
	return func(yield func(Elem) bool) {
		for p := range e.Elems() {
			if p.IsUnique() {
				if !yield(p) {
					break
				}
			}
		}
	}
}

func (e *protoEntity) Props() iter.Seq[Prop] {
	return slices.Values(e.props)
}

func (e *protoEntity) Fields() iter.Seq[Field] {
	return func(yield func(Field) bool) {
		for p := range e.Props() {
			if v, ok := p.(*protoField); ok {
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
			if v, ok := p.(*protoEdge); ok {
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

func (e *protoEntity) HasElems() bool {
	return e.HasProps() || e.HasIndexes()
}

func (e *protoEntity) HasProps() bool {
	return len(e.props) > 0
}

func (e *protoEntity) HasFields() (ok bool) {
	if !e.HasProps() {
		return false
	}

	e.Fields()(func(_ Field) bool {
		ok = true
		return false
	})

	return
}

func (e *protoEntity) HasEdges() (ok bool) {
	if !e.HasProps() {
		return false
	}

	e.Edges()(func(_ Edge) bool {
		ok = true
		return false
	})

	return
}

func (e *protoEntity) HasIndexes() bool {
	return len(e.indexes) > 0
}

func (e *protoEntity) HasVersionField() bool {
	return e.GetVersionField() != nil
}

func (e *protoEntity) GetVersionField() Field {
	for f := range e.Fields() {
		if f.IsVersion() {
			return f
		}
	}

	return nil
}
