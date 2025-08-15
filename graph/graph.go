package graph

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Graph struct {
	Entities map[protoreflect.FullName]Entity
}

func NewGraph() *Graph {
	return &Graph{
		Entities: map[protoreflect.FullName]Entity{},
	}
}

func (g *Graph) Clone() *Graph {
	return &Graph{
		Entities: maps.Clone(g.Entities),
	}
}

func (g *Graph) InPlaceMerge(h *Graph) {
	maps.Copy(g.Entities, h.Entities)
}

func Parse(ctx context.Context, g *Graph, f protoreflect.FileDescriptor) error {
	// TODO: overlay?
	g_ := g.Clone()
	errs := []error{}

	for i := 0; i < f.Messages().Len(); i++ {
		m := f.Messages().Get(i)
		om := proto.GetExtension(m.Options(), ormpb.E_Message).(*ormpb.MessageOptions)
		if om == nil {
			continue
		}
		if om.GetDisabled() {
			continue
		}

		v, err := parseEntity(ctx, g_, m, om)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s%w", m.FullName(), err))
			continue
		}

		g_.Entities[v.FullName()] = v
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	g.InPlaceMerge(g_)
	return nil
}

func ParseFiles(ctx context.Context, g *Graph, fs []*protogen.File) error {
	for _, f := range fs {
		if !f.Generate {
			continue
		}

		d := f.Desc
		if err := Parse(ctx, g, d); err != nil {
			return fmt.Errorf("parse %s: %w", d.Path(), err)
		}
	}

	return nil
}
