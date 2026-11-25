package graph

import (
	"errors"
	"fmt"

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

	// OnDelete is what becomes of this edge's row when the row it names is
	// deleted, and [ormpb.OnDelete_ON_DELETE_UNSPECIFIED] when the schema does
	// not say.
	//
	// Only an edge that holds the key can say it. An edge holds the key when it
	// is not a list, unless it is the To side of a one-to-one: an edge with no
	// [Edge.Inverse] whose [Edge.Reverse] is not a list, for then the key is
	// on the other side, the edge written `from` this one.
	//
	//	User.Parent, User.Children from User.Parent   Parent holds it
	//	Group.Members, Member.Group from Members      Group holds it
	//	Node.Next, Node.Prev from Node.Next           Prev holds it
	OnDelete() ormpb.OnDelete
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

func (e *protoEdge) OnDelete() ormpb.OnDelete {
	return e.opts.GetOnDelete()
}

// checkOnDelete refuses an on_delete the edge cannot carry, as [Edge.OnDelete]
// says. It runs once every entity of a file is parsed: [Edge.Reverse] reads
// the target's edges, and a target parsed on the way to this one may not have
// had them all yet.
func (e *protoEdge) checkOnDelete() error {
	v := e.OnDelete()
	if v == ormpb.OnDelete_ON_DELETE_UNSPECIFIED {
		return nil
	}
	if e.IsList() {
		return errors.New("on_delete is said on the edge that holds the key, and a list holds none")
	}
	if e.Inverse() == nil {
		if rev := e.Reverse(); rev != nil && !rev.IsList() {
			return fmt.Errorf("on_delete is said on the edge that holds the key, and of a one-to-one that is the side written from this one, %s; say it there", rev.FullName())
		}
	}
	if v == ormpb.OnDelete_ON_DELETE_SET_NULL && !e.IsNullable() {
		return errors.New("on_delete ON_DELETE_SET_NULL clears the edge, and this one is not nullable")
	}
	return nil
}
