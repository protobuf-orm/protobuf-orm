package graph

import (
	"context"
	"errors"
	"fmt"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Prop represents a property of an entity.
// It can be either a [Field] or an [Edge].
type Prop interface {
	// Entity which holds this prop.
	Entity() Entity
	Descriptor() protoreflect.FieldDescriptor

	Type() ormpb.Type
	FullName() protoreflect.FullName
	Name() string
	Number() protoreflect.FieldNumber

	HasDefault() bool

	IsList() bool
	IsUnique() bool
	IsNullable() bool
	IsImmutable() bool

	// IsOptional indicates that this prop is
	// does not have to be provided when this entity is created.
	// Returns `true` if any of the following is true:
	//  - IsNullable() returns true.
	//  - HasDefault() returns true.
	IsOptional() bool
}

type protoProp struct {
	// Proto field which this prop is based on.
	source protoreflect.FieldDescriptor
	// Entity which holds this prop.
	entity *protoEntity

	opts commonOpts
}

// Note that it does not find a inverse for the edge
// since not all props are parsed yet.
func parseProp(ctx context.Context, g *Graph, e *protoEntity, mf protoreflect.FieldDescriptor) (Prop, error) {
	of := proto.GetExtension(mf.Options(), ormpb.E_Field).(*ormpb.FieldOptions)
	oe := proto.GetExtension(mf.Options(), ormpb.E_Edge).(*ormpb.EdgeOptions)
	if of != nil && oe != nil {
		return nil, errors.New(`only one of "orm.field" or "orm.edge" can be specified`)
	}
	// The options returned by proto.GetExtension alias the descriptor's option
	// message, which must be treated as immutable. Clone them so the
	// normalization below (e.g. SetUnique/SetImmutable on the key) mutates a
	// private copy instead of shared, global descriptor state.
	if of != nil {
		of = proto.Clone(of).(*ormpb.FieldOptions)
	}
	if oe != nil {
		oe = proto.Clone(oe).(*ormpb.EdgeOptions)
	}
	if of.GetDisabled() || oe.GetDisabled() {
		return nil, nil
	}
	if (of == nil && oe == nil) || (of != nil && of.GetType() == ormpb.Type_TYPE_UNSPECIFIED) {
		// No option is specified for the prop
		// or no type is specified for the field so let's deduce it.
		t := ormpb.DeduceType(mf)
		if of == nil {
			of = &ormpb.FieldOptions{}
		}
		of.SetType(t)
	}
	if of.HasVersion() {
		// The key is checked on its own raw bit rather than through the two
		// below, because parseEntity marks the key unique and immutable in a
		// commit that runs after every prop is parsed -- long after this. A
		// field marked both key and version would pass here and then become
		// exactly the state the next line forbids, with no diagnostic: being
		// immutable it would drop out of the PatchRequest entirely, and the
		// lock would disappear from the RPC rather than fail loudly.
		if of.GetKey() {
			return nil, errors.New("version field cannot be the key")
		}
		if of.GetUnique() || of.GetNullable() || of.GetImmutable() {
			return nil, errors.New("version field cannot be unique, nullable or immutable")
		}
		if of.GetType() != ormpb.Type_TYPE_TIME {
			return nil, errors.New("currently, only the time type supports versioning")
		}
	}
	if of.GetType() == ormpb.Type_TYPE_MESSAGE {
		return nil, errors.New("field cannot be a message type (use JSON type instead)")
	}

	// TODO: do validation
	// e.g. if the kind is Bool, the type must be Bool.

	prop := protoProp{
		source: mf,
		entity: e,
	}

	// Prop must be ether one of field or edge.
	// `of` is set only and only if the prop is deduced as field and `oe` is nil.
	is_field := of != nil
	// is_edge := !is_field

	if is_field {
		prop.opts = of
		return &protoField{
			protoProp: prop,
			opts:      of,
		}, nil
	} else if oe == nil {
		oe = &ormpb.EdgeOptions{}
	}

	// An edge must reference another entity, so the underlying proto field has
	// to be a message. Guard here so that a scalar field marked as an edge
	// produces a clear error instead of a nil-pointer panic on mf.Message().
	if mf.Kind() != protoreflect.MessageKind {
		return nil, fmt.Errorf("edge must reference a message type (an entity), but field kind is %s", mf.Kind())
	}

	// Test if the reference is valid entity.
	target_name := mf.Message().FullName()
	target, ok := g.Entities[target_name]
	if !ok {
		target_, err := parseEntity(ctx, g, mf.Message())
		if err != nil {
			return nil, fmt.Errorf("parse target entity: %s%w", target_name, err)
		}
		if target_ == nil {
			return nil, fmt.Errorf("target is not an entity: %s", target_name)
		}

		// The target was in the same file.
		target = target_
	}
	if oe.GetUnique() && mf.Cardinality() == protoreflect.Repeated {
		return nil, fmt.Errorf("edge with repeated cardinality cannot be unique")
	}

	prop.opts = oe
	return &protoEdge{
		protoProp: prop,
		opts:      oe,
		target:    target,
	}, nil
}

func (p protoProp) Entity() Entity {
	return p.entity
}

func (p protoProp) Descriptor() protoreflect.FieldDescriptor {
	return p.source
}

func (p protoProp) Type() ormpb.Type {
	panic("not implemented")
}

func (p protoProp) FullName() protoreflect.FullName {
	return p.source.FullName()
}

func (p protoProp) Name() string {
	return string(p.source.FullName().Name())
}

func (p protoProp) Number() protoreflect.FieldNumber {
	return p.source.Number()
}

func (f protoProp) HasDefault() bool {
	return f.opts.HasDefault()
}

func (p protoProp) IsList() bool {
	return p.source.IsList()
}

func (f protoProp) IsUnique() bool {
	return f.opts.GetUnique()
}

func (f protoProp) isRepeated() bool {
	return f.source.Cardinality() == protoreflect.Repeated
}

func (f protoProp) IsImmutable() bool {
	return f.opts.GetImmutable()
}

type commonOpts interface {
	GetUnique() bool
	GetNullable() bool
	GetImmutable() bool
	HasDefault() bool
	GetDefault() string
}
