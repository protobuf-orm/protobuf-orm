package graph

import (
	"github.com/protobuf-orm/protobuf-orm/ormpb"
)

type Edge interface {
	Prop
	Target() Entity

	// Edge that referenced by this edge.
	//
	//	// User.Children to User
	//	// User.Parent from User.Children
	//	User.Children.Reverse() == User.Parent
	Reverse() Edge

	// Back-reference field for this edge.
	//
	//	// User.Children to User
	//	// User.Parent from User.Children
	//	User.Parent.Inverse() == User.Children
	Inverse() Edge
}

type protoEdge struct {
	protoProp
	opts *ormpb.EdgeOptions

	target  Entity
	inverse Edge
}

func (f *protoEdge) Type() ormpb.Type {
	return ormpb.Type_TYPE_MESSAGE
}

func (f *protoEdge) IsNullable() bool {
	if f.isRepeated() {
		// There is no way to distinguish between empty and null in proto.
		return false
	}

	//In proto, even if a message has explicit presence, an edge is not nullable;
	// it is only nullable when the nullable option is explicitly specified.
	return f.opts.GetNullable() ||
		f.source.HasOptionalKeyword()
}

func (f *protoEdge) IsOptional() bool {
	if f.isRepeated() {
		// Empty input for repeated prop is treated as an empty list or map.
		return true
	}
	return f.IsNullable() ||
		f.HasDefault()
}

func (e *protoEdge) Target() Entity {
	return e.target
}

func (e *protoEdge) Reverse() Edge {
	for v := range e.target.Edges() {
		inv := v.Inverse()
		if inv == nil {
			continue
		}
		if inv.FullName() == e.FullName() {
			return v
		}
	}

	return nil
}

func (e *protoEdge) Inverse() Edge {
	return e.inverse
}
