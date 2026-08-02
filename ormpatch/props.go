package ormpatch

import (
	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// props indexes an entity's props by field number.
//
// graph.Entity offers no lookup -- Props() is a sequence -- and a document
// addresses fields, so one index per compile is the cheapest way to answer
// "which column is this".
type props struct {
	ent   graph.Entity
	byNum map[protoreflect.FieldNumber]graph.Prop
}

func indexProps(e graph.Entity) *props {
	p := &props{ent: e, byNum: map[protoreflect.FieldNumber]graph.Prop{}}
	for v := range e.Props() {
		p.byNum[v.Number()] = v
	}
	return p
}

// resolve maps a document field reference onto a column of the entity.
//
// It defers identification to [patch.ResolveField] so that name/number/json_name
// agreement, extension ranges and the vacancy rule stay exactly as the format
// defines them, then maps the descriptor back onto a prop.
//
// The two steps are not the same question. Entity.Descriptor().Fields() is a
// strict superset of Props(): a field carrying orm.field.disabled is declared
// on the message but has no column. Such a field resolves here and then has no
// prop, which is not vacancy -- the message does declare it, so the format
// would happily write it -- and not a format error either. It is this engine
// declining, so it is [ErrUnsupported].
func (p *props) resolve(f *patchpb.Field, at patch.At) (graph.Prop, bool, error) {
	fd, vacant, err := patch.ResolveField(p.ent.Descriptor(), f, at)
	if err != nil {
		return nil, false, err
	}
	if vacant {
		return nil, true, nil
	}

	v, ok := p.byNum[fd.Number()]
	if !ok {
		return nil, false, unsupportedf(at,
			"%s is declared but is not mapped by the ORM, so the row has no "+
				"column for it", fd.FullName())
	}

	return v, false, nil
}

// isMap reports whether the prop is stored as a JSON object keyed by its map
// key -- true only for a map-typed Field, never for an edge.
func isMap(v graph.Prop) bool {
	f, ok := v.(graph.Field)
	if !ok {
		return false
	}
	return f.Descriptor().IsMap()
}

// isList reports whether the prop is stored as a JSON array. Mirrors
// protoreflect's rule that a map is not a list.
func isList(v graph.Prop) bool {
	return v.IsList()
}
