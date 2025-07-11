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

	FullName() protoreflect.FullName
	Number() protoreflect.FieldNumber

	IsUnique() bool
	IsNullable() bool
	IsImmutable() bool
}

type protoProp struct {
	// Proto field which this prop is based on.
	source protoreflect.FieldDescriptor
	// Entity which holds this prop.
	entity *protoEntity
}

// Note that it does not find a inverse for the edge
// since not all props are parsed yet.
func parseProp(ctx context.Context, g *Graph, e *protoEntity, mf protoreflect.FieldDescriptor) (Prop, error) {
	of := proto.GetExtension(mf.Options(), ormpb.E_Field).(*ormpb.FieldOptions)
	oe := proto.GetExtension(mf.Options(), ormpb.E_Edge).(*ormpb.EdgeOptions)
	if of != nil && oe != nil {
		return nil, errors.New(`only one of "orm.filed" or "orm.edge" can be specified`)
	}
	if of.GetDisabled() || oe.GetDisabled() {
		return nil, nil
	}
	if (of == nil && oe == nil) || of.GetType() == ormpb.Type_TYPE_UNSPECIFIED {
		// No option is specified for the prop
		// or no type is specified for the field so let's deduce it.
		t, err := ormpb.DeduceType(mf, of.GetType())
		if err != nil {
			return nil, fmt.Errorf("resolve type: %w", err)
		}
		if t == ormpb.Type_TYPE_MESSAGE {
			// Seems that the prop is an edge.
			if of != nil {
				// The user set it as message type explicitly.
				return nil, errors.New("field cannot be a message type (use JSON)")
			}
		} else {
			if of == nil {
				of = &ormpb.FieldOptions{}
			}
			of.SetType(t)
		}
	}

	prop := protoProp{
		source: mf,
		entity: e,
	}

	// Prop must be ether one of field or edge.
	// `of` is set only and only if the prop is deduced as field and `oe` is nil.
	is_field := of != nil
	// is_edge := !is_field

	if is_field {
		return &protoField{
			protoProp: prop,
			opts:      of,
		}, nil
	}

	// Test if the reference is valid entity.
	target_name := mf.Message().FullName()
	target, ok := g.Entities[target_name]
	if !ok {
		if e.source.ParentFile().Path() != mf.ParentFile().Path() {
			// Reference an entity outside of the current file
			// so if the parse is conducted in proper order,
			// the target entity must be in the graph.
			return nil, fmt.Errorf("target entity not found: %s", target_name)
		}

		// Seems that the target is defined in the same file
		// so try to parse the target first.
		// TODO:
		// target = parseEntity(...)
		panic("not implemented")
	}
	if mf.Cardinality() == protoreflect.Repeated && oe.GetUnique() {
		return nil, fmt.Errorf("edge with repeated cardinality cannot be unique")
	}

	return &protoEdge{
		protoProp: prop,
		opts:      oe,
		target:    target,
	}, nil
}

func (p protoProp) Entity() Entity {
	return p.entity
}

func (p protoProp) FullName() protoreflect.FullName {
	return p.source.FullName()
}

func (p protoProp) Number() protoreflect.FieldNumber {
	return p.source.Number()
}
