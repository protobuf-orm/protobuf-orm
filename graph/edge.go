package graph

import "github.com/protobuf-orm/protobuf-orm/ormpb"

type Edge interface {
	Prop
	Target() Entity
}

type protoEdge struct {
	protoProp
	opts   *ormpb.EdgeOptions
	target Entity
}

func (e *protoEdge) Target() Entity {
	return e.target
}
