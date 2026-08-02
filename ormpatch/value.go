package ormpatch

import (
	"github.com/lesomnus/protobuf-patch/patch"
	"github.com/lesomnus/protobuf-patch/patchpb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// materialize turns a document literal into the value a column would hold.
//
// fd is always the FIELD, never a resolved element or map value: [patch.Site]
// says which part of it the literal lands on, and [patch.CheckArm] derives the
// governing kind from the pair. Handing it an already-resolved descriptor makes
// it resolve twice.
//
// [patch.Scalar] does the leaves, including the arm-vs-kind table and the
// closed-enum check, so this only adds the shapes that need somewhere to
// allocate into.
func materialize(v *patchpb.Value, fd protoreflect.FieldDescriptor, site patch.Site, at patch.At) (protoreflect.Value, error) {
	if err := patch.CheckArm(v, fd, site, at); err != nil {
		return protoreflect.Value{}, err
	}

	switch patch.ShapeOf(v) {
	case patch.ShapeScalar:
		return patch.Scalar(v, fd, site, at)

	case patch.ShapeMessage:
		md := fd.Message()
		if site == patch.SiteMapValue {
			md = fd.MapValue().Message()
		}
		m := dynamicpb.NewMessage(md)
		if err := fillMessage(m, v.GetM(), at.Sub("m")); err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(m), nil

	case patch.ShapeList:
		// A collection needs a live field to allocate into, and CheckArm only
		// admits `l` for the whole field, so fd is the list itself here.
		host := dynamicpb.NewMessage(fd.ContainingMessage())
		lv := host.NewField(fd)
		if err := fillList(lv.List(), fd, v.GetL(), at.Sub("l")); err != nil {
			return protoreflect.Value{}, err
		}
		return lv, nil

	case patch.ShapeMap:
		host := dynamicpb.NewMessage(fd.ContainingMessage())
		mv := host.NewField(fd)
		if err := fillMap(mv.Map(), fd, v.GetMap(), at.Sub("map")); err != nil {
			return protoreflect.Value{}, err
		}
		return mv, nil
	}

	// ShapeNone. Validate has already refused an unset arm, so reaching here
	// means a newer revision added one -- fail closed rather than write nothing.
	return protoreflect.Value{}, patch.Errf(patch.CodeIllegalArm, at, "unrecognized value arm")
}

func fillMessage(m protoreflect.Message, mv *patchpb.MessageValue, at patch.At) error {
	md := m.Descriptor()
	seen := map[protoreflect.FieldNumber]bool{}
	oneofs := map[protoreflect.FullName]bool{}

	for i, fv := range mv.GetFields() {
		at := at.Index("fields", i)

		fd, vacant, err := patch.ResolveField(md, fv.GetKey(), at.Sub("key"))
		if err != nil {
			return err
		}
		if vacant {
			// A value position never tolerates vacancy, whatever on_missing
			// says -- the literal names a field the message does not declare.
			return patch.Errf(patch.CodeVacantTarget, at.Sub("key"),
				"%s declares no such field", md.FullName())
		}
		if seen[fd.Number()] {
			return patch.Errf(patch.CodeDuplicateTarget, at.Sub("key"),
				"%s is set twice", fd.FullName())
		}
		seen[fd.Number()] = true

		if od := fd.ContainingOneof(); od != nil && !od.IsSynthetic() {
			if oneofs[od.FullName()] {
				return patch.Errf(patch.CodeDuplicateTarget, at.Sub("key"),
					"two members of oneof %s", od.FullName())
			}
			oneofs[od.FullName()] = true
		}

		pv, err := materialize(fv.GetValue(), fd, patch.SiteField, at.Sub("value"))
		if err != nil {
			return err
		}
		m.Set(fd, pv)
	}

	return nil
}

func fillList(l protoreflect.List, fd protoreflect.FieldDescriptor, lv *patchpb.ListValue, at patch.At) error {
	for i, ev := range lv.GetValues() {
		at := at.Index("values", i)

		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			em := l.NewElement()
			if err := patch.CheckArm(ev, fd, patch.SiteElement, at); err != nil {
				return err
			}
			if patch.ShapeOf(ev) != patch.ShapeMessage {
				return patch.Errf(patch.CodeIllegalArm, at, "expected a message element")
			}
			if err := fillMessage(em.Message(), ev.GetM(), at.Sub("m")); err != nil {
				return err
			}
			l.Append(em)
			continue
		}

		pv, err := patch.Scalar(ev, fd, patch.SiteElement, at)
		if err != nil {
			return err
		}
		l.Append(pv)
	}

	return nil
}

func fillMap(mp protoreflect.Map, fd protoreflect.FieldDescriptor, mv *patchpb.MapValue, at patch.At) error {
	vd := fd.MapValue()
	seen := map[any]bool{}

	for i, ev := range mv.GetEntries() {
		at := at.Index("entries", i)

		mk, err := patch.MapKeyFor(ev.GetKey(), fd, at.Sub("key"))
		if err != nil {
			return err
		}
		if seen[mk.Interface()] {
			return patch.Errf(patch.CodeDuplicateTarget, at.Sub("key"),
				"key %v appears twice", mk.Interface())
		}
		seen[mk.Interface()] = true

		if vd.Kind() == protoreflect.MessageKind || vd.Kind() == protoreflect.GroupKind {
			em := mp.NewValue()
			if err := patch.CheckArm(ev.GetValue(), fd, patch.SiteMapValue, at.Sub("value")); err != nil {
				return err
			}
			if patch.ShapeOf(ev.GetValue()) != patch.ShapeMessage {
				return patch.Errf(patch.CodeIllegalArm, at.Sub("value"), "expected a message value")
			}
			if err := fillMessage(em.Message(), ev.GetValue().GetM(), at.Sub("value").Sub("m")); err != nil {
				return err
			}
			mp.Set(mk, em)
			continue
		}

		pv, err := patch.Scalar(ev.GetValue(), fd, patch.SiteMapValue, at.Sub("value"))
		if err != nil {
			return err
		}
		mp.Set(mk, pv)
	}

	return nil
}
