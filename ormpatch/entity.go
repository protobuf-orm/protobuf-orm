package ormpatch

import (
	"context"
	"fmt"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// EntityOf parses one file and returns the entity named in it.
//
// Compiling a document needs the schema, and a generated server has only the
// descriptors it was built from. This is the bridge: call it once at package
// level rather than per request, since parsing walks the whole file.
func EntityOf(fd protoreflect.FileDescriptor, name protoreflect.Name) (graph.Entity, error) {
	g := graph.NewGraph()
	if err := graph.Parse(context.Background(), g, fd); err != nil {
		return nil, fmt.Errorf("parse %s: %w", fd.Path(), err)
	}

	e, ok := g.Entities[fd.Package().Append(name)]
	if !ok {
		return nil, fmt.Errorf("%s declares no entity %s", fd.Path(), name)
	}

	return e, nil
}

// MustEntityOf is [EntityOf], panicking on error.
//
// Intended for a package-level variable in generated code, where a failure
// means the generator and the schema disagree -- a build-time bug, not
// something a request can recover from.
func MustEntityOf(fd protoreflect.FileDescriptor, name protoreflect.Name) graph.Entity {
	e, err := EntityOf(fd, name)
	if err != nil {
		panic(err)
	}
	return e
}
