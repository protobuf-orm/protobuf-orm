package ormpb

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func RegByNumber(n int) *Ref {
	return Ref_builder{Number: int32(n)}.Build()
}

func RefByName(name string) *Ref {
	return Ref_builder{Name: name}.Build()
}

func (r *Ref) Access(desc protoreflect.FieldDescriptors) (protoreflect.FieldDescriptor, error) {
	number := r.GetNumber()
	name := r.GetName()
	if number == 0 && name == "" {
		return nil, errors.New("empty ref")
	}

	var field protoreflect.FieldDescriptor
	if number > 0 {
		field = desc.ByNumber(protoreflect.FieldNumber(number))
		if field == nil {
			return nil, fmt.Errorf("unknown number: %d", number)
		}

		name_ := string(field.Name())
		if name != name_ {
			return nil, fmt.Errorf("name not matched with the number: expected %d:%q but was %q", number, name_, name)
		}
	} else {
		// name != ""
		field = desc.ByName(protoreflect.Name(name))
		if field == nil {
			return nil, fmt.Errorf("unknown name: %q", name)
		}
	}

	return field, nil
}
