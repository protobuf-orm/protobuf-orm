package graph

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Graph is the parsed schema model: the set of entities (keyed by their proto
// full name) resolved from one or more proto files.
type Graph struct {
	Entities map[protoreflect.FullName]Entity
}

// NewGraph returns an empty graph ready to be populated with [Parse] or
// [ParseFiles].
func NewGraph() *Graph {
	return &Graph{
		Entities: map[protoreflect.FullName]Entity{},
	}
}

// Clone returns a shallow copy of the graph: the entity map is duplicated but
// the entity values are shared. It is used by [Parse] to stage a file's
// entities so a failed parse does not mutate the original graph.
func (g *Graph) Clone() *Graph {
	return &Graph{
		Entities: maps.Clone(g.Entities),
	}
}

// InPlaceMerge copies every entity from h into g, overwriting entries with the
// same name.
func (g *Graph) InPlaceMerge(h *Graph) {
	maps.Copy(g.Entities, h.Entities)
}

// Parse resolves every entity declared in the file descriptor f and merges them
// into g. Messages without the orm.message option are skipped. Parsing is
// transactional: it stages results in a clone and only merges into g if the
// whole file parses without error, so a failed Parse never leaves a partially
// built entity in g. Call Parse repeatedly to accumulate entities from several
// files (which is how cross-file relations resolve).
func Parse(ctx context.Context, g *Graph, f protoreflect.FileDescriptor) error {
	// TODO: overlay?
	g_ := g.Clone()
	errs := []error{}

	for i := 0; i < f.Messages().Len(); i++ {
		m := f.Messages().Get(i)

		v, err := parseEntity(ctx, g_, m)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s%w", m.FullName(), err))
			continue
		}
		if v == nil {
			// Not an Entity
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

// ParseFiles parses every file marked for generation, in order, into g. It is
// the entry point for a protoc/buf plugin, which passes gen.Files. Files not
// marked for generation (imports) are skipped.
func ParseFiles(ctx context.Context, g *Graph, fs []*protogen.File) error {
	for _, f := range fs {
		if !f.Generate {
			continue
		}

		d := f.Desc
		if err := Parse(ctx, g, d); err != nil {
			return fmt.Errorf("%s: %w", d.Path(), err)
		}
	}

	return nil
}
