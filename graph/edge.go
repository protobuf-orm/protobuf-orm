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
