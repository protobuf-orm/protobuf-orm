// Machinery shared by the FromPatchRequest tests.
//
// A generated PatchRequest is what the converter reads, and protobuf-orm
// generates none: the message belongs to protoc-gen-orm-service. So the request
// descriptor is SYNTHESIZED here from the entity's own descriptor by applying
// the layout in [graph.PatchLayout], which makes these tests an executable
// statement of that convention as much as a harness for the converter.

package ormpatch_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"github.com/protobuf-orm/protobuf-orm/graph"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/graphtest"
	"github.com/protobuf-orm/protobuf-orm/internal/examples/library"
	"github.com/protobuf-orm/protobuf-orm/ormpatch"
	"github.com/protobuf-orm/protobuf-orm/ormpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const probePkg = "zzprobe"

// buildPatchRequest derives the XxxPatchRequest descriptor from the entity's
// own descriptor by the 2N-1 / 2N rule, exactly as protoc-gen-orm-service does
// when it emits the .proto.
func buildPatchRequest(e graph.Entity) (protoreflect.MessageDescriptor, error) {
	msgName := e.Name() + "PatchRequest"

	deps := map[string]bool{e.Descriptor().ParentFile().Path(): true}
	addDep := func(fd protoreflect.FieldDescriptor) {
		switch fd.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			deps[fd.Message().ParentFile().Path()] = true
		case protoreflect.EnumKind:
			deps[fd.Enum().ParentFile().Path()] = true
		}
	}

	refs := map[string]*descriptorpb.DescriptorProto{}
	refFor := func(t graph.Entity) string {
		name := t.Name() + "Ref"
		if _, ok := refs[name]; !ok {
			kfd := t.Key().Descriptor()
			addDep(kfd)
			k := protodesc.ToFieldDescriptorProto(kfd)
			k.Options = nil
			refs[name] = &descriptorpb.DescriptorProto{
				Name:  proto.String(name),
				Field: []*descriptorpb.FieldDescriptorProto{k},
			}
		}
		return "." + probePkg + "." + name
	}

	msg := &descriptorpb.DescriptorProto{Name: proto.String(msgName)}

	// Every number here comes from graph, the same place protoc-gen-orm-service
	// takes it from. Re-deriving the arithmetic would make this fixture agree
	// with the converter by coincidence rather than by construction, and it is
	// supposed to stand in for the real generator.
	msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
		Name:     proto.String("ref"),
		Number:   proto.Int32(int32(graph.PatchRefNumber(e))),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: proto.String(refFor(e)),
	})

	for p := range graph.PatchProps(e) {
		var fp *descriptorpb.FieldDescriptorProto
		switch p := p.(type) {
		case graph.Edge:
			label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
			if p.IsList() {
				label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
			}
			fp = &descriptorpb.FieldDescriptorProto{
				Name:     proto.String(p.Name()),
				Label:    label.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(refFor(p.Target())),
			}

		case graph.Field:
			fd := p.Descriptor()
			addDep(fd)
			fp = protodesc.ToFieldDescriptorProto(fd)
			fp.Options = nil
			fp.JsonName = nil
			fp.OneofIndex = nil
			fp.Proto3Optional = nil
			if fd.IsMap() {
				// Inline the map entry rather than pointing across files at the
				// entity's own nested entry type.
				entry := protodesc.ToDescriptorProto(fd.Message())
				for _, ef := range entry.Field {
					ef.Options = nil
					ef.JsonName = nil
				}
				if mv := fd.MapValue(); mv != nil {
					addDep(mv)
				}
				msg.NestedType = append(msg.NestedType, entry)
				fp.TypeName = proto.String(
					"." + probePkg + "." + msgName + "." + entry.GetName())
			}

		default:
			return nil, fmt.Errorf("unknown prop type %T", p)
		}

		fp.Number = proto.Int32(int32(graph.PatchValueNumber(p)))
		msg.Field = append(msg.Field, fp)

		if name := graph.PatchFlagName(p); name != "" {
			msg.Field = append(msg.Field, boolField(name, int32(graph.PatchFlagNumber(p))))
		}
	}

	file := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("zzprobe/" + string(e.Name()) + "_patch_request.proto"),
		Package:    proto.String(probePkg),
		Syntax:     proto.String("editions"),
		Edition:    descriptorpb.Edition_EDITION_2023.Enum(),
		Dependency: slices.Sorted(maps(deps)),
		Options:    &descriptorpb.FileOptions{GoPackage: proto.String(probePkg)},
	}
	for _, name := range slices.Sorted(maps(refs)) {
		file.MessageType = append(file.MessageType, refs[name])
	}
	file.MessageType = append(file.MessageType, msg)

	fd, err := protodesc.NewFile(file, entityResolver{own: e.Descriptor().ParentFile()})
	if err != nil {
		return nil, err
	}
	md := fd.Messages().ByName(protoreflect.Name(msgName))
	if md == nil {
		return nil, fmt.Errorf("%s not built", msgName)
	}
	return md, nil
}

// entityResolver resolves against the global registry, falling back to the
// entity's OWN file -- which, for a probe, may have been synthesized and never
// registered.
type entityResolver struct{ own protoreflect.FileDescriptor }

func (r entityResolver) FindFileByPath(p string) (protoreflect.FileDescriptor, error) {
	if r.own != nil && r.own.Path() == p {
		return r.own, nil
	}
	return protoregistry.GlobalFiles.FindFileByPath(p)
}

func (r entityResolver) FindDescriptorByName(n protoreflect.FullName) (protoreflect.Descriptor, error) {
	if r.own != nil {
		if d, ok := findInFile(r.own, n); ok {
			return d, nil
		}
	}
	return protoregistry.GlobalFiles.FindDescriptorByName(n)
}

func findInFile(f protoreflect.FileDescriptor, n protoreflect.FullName) (protoreflect.Descriptor, bool) {
	var walk func(protoreflect.MessageDescriptors) (protoreflect.Descriptor, bool)
	walk = func(ms protoreflect.MessageDescriptors) (protoreflect.Descriptor, bool) {
		for i := range ms.Len() {
			m := ms.Get(i)
			if m.FullName() == n {
				return m, true
			}
			if d, ok := walk(m.Messages()); ok {
				return d, true
			}
		}
		return nil, false
	}
	if d, ok := walk(f.Messages()); ok {
		return d, true
	}
	for i := range f.Enums().Len() {
		if e := f.Enums().Get(i); e.FullName() == n {
			return e, true
		}
	}
	return nil, false
}

func boolField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
	}
}

func maps[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TEST HARNESS
// ---------------------------------------------------------------------------

type probe struct {
	x   *require.Assertions
	e   graph.Entity
	md  protoreflect.MessageDescriptor
	req protoreflect.Message
}

func (p *probe) fd(name string) protoreflect.FieldDescriptor {
	fd := p.md.Fields().ByName(protoreflect.Name(name))
	p.x.NotNil(fd, "request %s has no field %q; it has %s", p.md.FullName(), name, p.layout())
	return fd
}

func (p *probe) layout() string {
	out := ""
	for i := range p.md.Fields().Len() {
		fd := p.md.Fields().Get(i)
		out += fmt.Sprintf("\n\t%3d %-24s %s", fd.Number(), fd.Name(), kindOf(fd))
	}
	return out
}

func kindOf(fd protoreflect.FieldDescriptor) string {
	switch {
	case fd.IsMap():
		return fmt.Sprintf("map<%v,%v>", fd.MapKey().Kind(), fd.MapValue().Kind())
	case fd.IsList():
		return "repeated " + fd.Kind().String()
	case fd.Kind() == protoreflect.MessageKind:
		return string(fd.Message().FullName())
	default:
		return fd.Kind().String()
	}
}

func (p *probe) set(name string, v protoreflect.Value) *probe {
	p.req.Set(p.fd(name), v)
	return p
}

func (p *probe) setMap(name string, kv map[string]string) *probe {
	m := p.req.Mutable(p.fd(name)).Map()
	for k, v := range kv {
		m.Set(protoreflect.ValueOfString(k).MapKey(), protoreflect.ValueOfString(v))
	}
	return p
}

func (p *probe) setTime(name string, seconds int64, nanos int32) *probe {
	fd := p.fd(name)
	ts := p.req.NewField(fd).Message()
	ts.Set(ts.Descriptor().Fields().ByName("seconds"), protoreflect.ValueOfInt64(seconds))
	if nanos != 0 {
		ts.Set(ts.Descriptor().Fields().ByName("nanos"), protoreflect.ValueOfInt32(nanos))
	}
	p.req.Set(fd, protoreflect.ValueOfMessage(ts))
	return p
}

// setRef fills a Ref message by its key field name.
func (p *probe) setRef(name string, keyField string, v protoreflect.Value) *probe {
	fd := p.fd(name)
	r := p.req.NewField(fd).Message()
	r.Set(r.Descriptor().Fields().ByName(protoreflect.Name(keyField)), v)
	p.req.Set(fd, protoreflect.ValueOfMessage(r))
	return p
}

func (p *probe) convert(resolve ormpatch.EdgeResolver) (*patchpb.Patch, error) {
	return ormpatch.FromPatchRequest(p.e, p.req, resolve)
}

// run converts, compiles, and returns the plan.
func (p *probe) run(resolve ormpatch.EdgeResolver) *ormpatch.Plan {
	doc, err := p.convert(resolve)
	p.x.NoError(err)
	p.x.NotNil(doc, "converter produced no document")

	// Redundant on the delegation path (CompileWith validates first) but it
	// makes a malformed template fail here instead of downstream.
	p.x.NoError(patch.Validate(doc))

	plan, err := ormpatch.Compile(p.e, doc)
	p.x.NoError(err, "document was:\n%v", doc)
	return plan
}

func withProbe(file protoreflect.FileDescriptor, name string, f func(p *probe)) func(*testing.T) {
	return func(t *testing.T) {
		x := require.New(t)

		g := graph.NewGraph()
		x.NoError(graph.Parse(context.Background(), g, file))

		md := file.Messages().ByName(protoreflect.Name(name))
		x.NotNil(md)
		e, ok := g.Entities[md.FullName()]
		x.True(ok, "entities: %v", g.Entities)

		rmd, err := buildPatchRequest(e)
		x.NoError(err)

		f(&probe{x: x, e: e, md: rmd, req: dynamicpb.NewMessage(rmd)})
	}
}

func withUser(f func(p *probe)) func(*testing.T) {
	return withProbe(library.File_library_user_proto, "User", f)
}

func withVersioned(f func(p *probe)) func(*testing.T) {
	return withProbe(graphtest.File_graphtest_version_proto, "VersionField", f)
}

func writeTo(x *require.Assertions, plan *ormpatch.Plan, name string) ormpatch.Write {
	w, ok := findWrite(plan, name)
	if !ok {
		x.FailNow("no write to " + name)
	}
	return w
}

func findWrite(plan *ormpatch.Plan, name string) (ormpatch.Write, bool) {
	for _, w := range plan.Writes {
		if w.Prop.Name() == name {
			return w, true
		}
	}
	return ormpatch.Write{}, false
}

// ---------------------------------------------------------------------------
// THE LAYOUT ITSELF
// ---------------------------------------------------------------------------

func hazardEntity(x *require.Assertions, name string, fields []*descriptorpb.FieldDescriptorProto) graph.Entity {
	mopts := &descriptorpb.MessageOptions{}
	proto.SetExtension(mopts, ormpb.E_Message, &ormpb.MessageOptions{})

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("zzprobe/hazard_" + name + ".proto"),
		Package: proto.String("zzprobe.hazard"),
		Syntax:  proto.String("editions"),
		Edition: descriptorpb.Edition_EDITION_2023.Enum(),
		Dependency: []string{
			"google/protobuf/timestamp.proto",
			string(ormpb.E_Field.TypeDescriptor().ParentFile().Path()),
		},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name:    proto.String(name),
			Field:   fields,
			Options: mopts,
		}},
	}

	fd, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
	x.NoError(err)

	g := graph.NewGraph()
	x.NoError(graph.Parse(context.Background(), g, fd))

	e, ok := g.Entities[fd.Messages().Get(0).FullName()]
	x.True(ok, "entities: %v", g.Entities)
	return e
}

func ormField(
	name string,
	number int32,
	kind descriptorpb.FieldDescriptorProto_Type,
	opts *ormpb.FieldOptions,
) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   kind.Enum(),
	}
	if opts != nil {
		f.Options = &descriptorpb.FieldOptions{}
		proto.SetExtension(f.Options, ormpb.E_Field, opts)
	}
	return f
}

func ormTimestamp(
	name string,
	number int32,
	opts *ormpb.FieldOptions,
) *descriptorpb.FieldDescriptorProto {
	f := ormField(name, number, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, opts)
	f.TypeName = proto.String(".google.protobuf.Timestamp")
	return f
}

// ---------------------------------------------------------------------------
// THE CONVERSION
// ---------------------------------------------------------------------------
