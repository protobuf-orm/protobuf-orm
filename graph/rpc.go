package graph

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type RpcMap interface {
	HasAdd() bool
	GetAdd() Rpc
	// HasAddMany is whether there is an AddMany beside the Add, which there
	// can only be with one.
	HasAddMany() bool
	GetAddMany() RpcAddMany
	HasGet() bool
	GetGet() Rpc
	HasPatch() bool
	GetPatch() Rpc
	// HasApply is whether there is an Apply, which writes a patch document to
	// a row. It is asked for on its own and does not come with the Patch.
	HasApply() bool
	GetApply() Rpc
	HasErase() bool
	GetErase() Rpc
}

type rpcMap struct {
	add     *rpcAdd
	addMany *rpcAddMany
	get     *rpcGet
	patch   *rpcPatch
	apply   *rpcApply
	erase   *rpcErase
}

func (m *rpcMap) HasAdd() bool   { return m.add != nil }
func (m *rpcMap) GetAdd() Rpc    { return m.add }
func (m *rpcMap) HasGet() bool   { return m.get != nil }
func (m *rpcMap) GetGet() Rpc    { return m.get }
func (m *rpcMap) HasPatch() bool { return m.patch != nil }
func (m *rpcMap) GetPatch() Rpc  { return m.patch }
func (m *rpcMap) HasErase() bool { return m.erase != nil }
func (m *rpcMap) GetErase() Rpc  { return m.erase }

func (m *rpcMap) HasApply() bool { return m.apply != nil }

// GetApply answers nil, and not a nil *rpcApply, when there is none.
func (m *rpcMap) GetApply() Rpc {
	if m.apply == nil {
		return nil
	}
	return m.apply
}

func (m *rpcMap) HasAddMany() bool { return m.addMany != nil }

// GetAddMany answers nil, and not a nil RpcAddMany, when there is none, so
// that `GetAddMany() == nil` says what it looks like it says.
func (m *rpcMap) GetAddMany() RpcAddMany {
	if m.addMany == nil {
		return nil
	}
	return m.addMany
}

type Rpc interface {
	Entity() Entity

	FullName() protoreflect.FullName

	Request() RpcMessage
	Response() RpcMessage
}

type rpc struct {
	entity   *protoEntity
	fullname protoreflect.FullName

	req *rpcMessage
	res *rpcMessage
}

// parseRpcs reads opts and does not write to it: opts is the schema's own,
// shared by every reader of the descriptor.
//
// The names are the ones protoc-gen-orm-service writes, request and answer
// for every verb.
func parseRpcs(ctx context.Context, g *Graph, e *protoEntity, opts *ormpb.RpcOptions) (*rpcMap, error) {
	if err := rpcOptionsKnown(opts); err != nil {
		return nil, err
	}

	rpcs := &rpcMap{}
	if opts.GetDisabled() {
		return rpcs, nil
	}

	pkg_name := e.FullName().Parent()                 // e.g. library
	msg_name := e.FullName().Name()                   // e.g. User
	svc_name := pkg_name.Append(msg_name + "Service") // e.g. library.UserService

	if opt := opts.GetAdd(); opts.HasAdd() && !opt.GetDisabled() {
		rpcs.add = &rpcAdd{
			rpc: rpc{
				entity:   e,
				fullname: svc_name.Append("Add"),

				req: &rpcMessage{fullname: pkg_name.Append(msg_name + "AddRequest")},
				res: &rpcMessage{fullname: e.FullName()},
			},
			opt: opt,
		}

		// Inside the Add, because AddMany is that Add many times: an entity
		// whose Add is disabled has no single row to add, let alone many.
		if many := opt.GetMany(); opt.HasMany() {
			rpcs.addMany = &rpcAddMany{
				rpc: rpc{
					entity:   e,
					fullname: svc_name.Append("AddMany"),

					req: &rpcMessage{fullname: pkg_name.Append(msg_name + "AddManyRequest")},
					res: &rpcMessage{fullname: pkg_name.Append(msg_name + "AddManyResponse")},
				},
				opt: many,
			}
		}
	}
	if opt := opts.GetGet(); opts.HasGet() && !opt.GetDisabled() {
		rpcs.get = &rpcGet{
			rpc: rpc{
				entity:   e,
				fullname: svc_name.Append("Get"),

				req: &rpcMessage{fullname: pkg_name.Append(msg_name + "GetRequest")},
				res: &rpcMessage{fullname: e.FullName()},
			},
			opt: opt,
		}
	}
	if opt := opts.GetPatch(); opts.HasPatch() && !opt.GetDisabled() {
		rpcs.patch = &rpcPatch{
			rpc: rpc{
				entity:   e,
				fullname: svc_name.Append("Patch"),

				req: &rpcMessage{fullname: pkg_name.Append(msg_name + "PatchRequest")},
				res: &rpcMessage{fullname: e.FullName()},
			},
			opt: opt,
		}
	}
	if opt := opts.GetApply(); opts.HasApply() && !opt.GetDisabled() {
		rpcs.apply = &rpcApply{
			rpc: rpc{
				entity:   e,
				fullname: svc_name.Append("Apply"),

				req: &rpcMessage{fullname: pkg_name.Append(msg_name + "ApplyRequest")},
				res: &rpcMessage{fullname: e.FullName()},
			},
			opt: opt,
		}
	}
	if opt := opts.GetErase(); opts.HasErase() && !opt.GetDisabled() {
		rpcs.erase = &rpcErase{
			rpc: rpc{
				entity:   e,
				fullname: svc_name.Append("Erase"),

				req: &rpcMessage{fullname: pkg_name.Append(msg_name + "Ref")},
				res: &rpcMessage{fullname: pkg_name.Append(msg_name + "EraseResponse")},
			},
			opt: opt,
		}
	}

	return rpcs, nil
}

// rpcOptionsKnown refuses an RpcOptions that carries a field this `orm` has no
// name for.
//
// Such a field is dropped by everything that reads the message, so what it
// asked for would quietly not happen. That matters most for `crud`: a schema
// compiled against an `orm` that still had it arrives here with field 2 set
// and nothing else, which would read as an entity with no verb at all.
func rpcOptionsKnown(opts *ormpb.RpcOptions) error {
	unknown := []protowire.Number{}
	for b := opts.ProtoReflect().GetUnknown(); len(b) > 0; {
		n, t, l := protowire.ConsumeTag(b)
		if l < 0 {
			return fmt.Errorf(": rpc: %w", protowire.ParseError(l))
		}
		b = b[l:]
		if l = protowire.ConsumeFieldValue(n, t, b); l < 0 {
			return fmt.Errorf(": rpc: %w", protowire.ParseError(l))
		}
		b = b[l:]
		unknown = append(unknown, n)
	}
	if len(unknown) == 0 {
		return nil
	}
	if slices.Contains(unknown, 2) {
		return errors.New(": rpc: crud is gone; name each verb the entity has (add, get, patch, apply, erase), and Apply no longer comes with Patch")
	}
	return fmt.Errorf(": rpc: field %d is unknown; the schema was compiled against another orm than this generator was built with", unknown[0])
}

func (r *rpc) Entity() Entity {
	return r.entity
}

func (r *rpc) FullName() protoreflect.FullName {
	return r.fullname
}

func (r *rpc) Request() RpcMessage {
	return r.req
}

func (r *rpc) Response() RpcMessage {
	return r.res
}

type rpcAdd struct {
	rpc
	opt *ormpb.RpcAdd
}

// DefaultAddManyMax is the most items an AddMany carries when its option says
// nothing.
//
// A thousand because it is one transaction, which holds its locks until the
// last row is written, and because it is well inside what a statement can bind
// on any engine once a generator splits the rows into statements of a few
// hundred.
const DefaultAddManyMax = 1000

// RpcAddMany is AddMany, which adds many rows of the entity in one call: a list
// of the entity's Add requests in, the rows they added out, in the same order.
type RpcAddMany interface {
	Rpc

	// Max is the most items one call may carry. A call with more is refused.
	Max() int
}

type rpcAddMany struct {
	rpc
	opt *ormpb.RpcAddMany
}

func (r *rpcAddMany) Max() int {
	if v := r.opt.GetMax(); v > 0 {
		return int(v)
	}
	return DefaultAddManyMax
}

type rpcGet struct {
	rpc
	opt *ormpb.RpcGet
}

type rpcPatch struct {
	rpc
	opt *ormpb.RpcPatch
}

type rpcApply struct {
	rpc
	opt *ormpb.RpcApply
}

type rpcErase struct {
	rpc
	opt *ormpb.RpcErase
}

type RpcMessage interface {
	FullName() protoreflect.FullName
	IsStream() bool
}

type rpcMessage struct {
	fullname protoreflect.FullName
	stream   bool
}

func (r *rpcMessage) FullName() protoreflect.FullName {
	return r.fullname
}

func (r *rpcMessage) IsStream() bool {
	return r.stream
}
