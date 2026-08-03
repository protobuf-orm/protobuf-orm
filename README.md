# protobuf-orm

Define your data model **once**, in Protocol Buffers, and let `protobuf-orm`
turn it into a rich schema graph that code generators can walk to emit ORM
code, database schemas, and gRPC services.

You annotate ordinary proto messages with a small set of custom options
(`orm.message`, `orm.field`, `orm.edge`). `protobuf-orm` parses the resulting
descriptors into an in-memory **graph** of entities, fields, edges (relations),
indexes, keys, and RPCs — fully resolved and validated.

> **Status:** early / in active development. The library currently builds and
> validates the schema graph; it is the foundation for code generators built on
> top of it. APIs may change.

- **What it is:** a schema-modeling library for protoc/buf plugins.
- **What it is not:** a runtime database driver or query builder.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the internal design.

## Contents

- [Concepts](#concepts)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Defining a schema](#defining-a-schema)
  - [Entities](#entities)
  - [Fields](#fields)
  - [Field types](#field-types)
  - [Nullable & optional](#nullable--optional)
  - [Keys](#keys)
  - [Edges (relations)](#edges-relations)
  - [Indexes](#indexes)
  - [Version fields (optimistic locking)](#version-fields-optimistic-locking)
  - [RPCs](#rpcs)
- [Using the graph](#using-the-graph)
- [Public API](#public-api)
- [Development](#development)

## Concepts

| Concept     | Proto annotation              | Meaning                                                      |
| ----------- | ----------------------------- | ----------------------------------------------------------- |
| **Entity**  | `option (orm.message)`        | A message that maps to a table/record.                      |
| **Field**   | `[(orm.field) = {...}]`       | A scalar / JSON / UUID / time column.                       |
| **Edge**    | `[(orm.edge) = {...}]`        | A relation to another entity.                               |
| **Key**     | `(orm.field) = {key: true}`   | The entity's primary key (implicitly unique & immutable).   |
| **Index**   | `(orm.message).indexes`       | A (optionally unique) index over one or more props.         |
| **RPC**     | `(orm.message).rpc`           | Generated `Add` / `Get` / `Patch` / `Erase` operations.     |

A **Prop** is the umbrella term for "a member of an entity" — every prop is
either a Field or an Edge.

## Installation

```sh
go get github.com/protobuf-orm/protobuf-orm
```

The custom options live in the `orm` proto package (`proto/orm/*.proto`,
go-package `ormpb`). To annotate your own protos you import `orm.proto`.

## Quick start

Annotate a message:

```proto
edition = "2023";
package library;

import "google/protobuf/timestamp.proto";
import "orm.proto";

option go_package = "example.com/library";

message User {
  bytes id = 1 [(orm.field) = {type: TYPE_UUID, key: true, default: ""}];
  string alias = 4 [(orm.field) = {unique: true, default: ""}];
  string name = 5;                       // implicitly TYPE_STRING

  google.protobuf.Timestamp date_created = 15 [(orm.field) = {
    immutable: true
    default: ""
  }];

  option (orm.message) = {rpc: {crud: true}};
}
```

Parse it into a graph and walk it:

```go
package main

import (
	"context"
	"fmt"

	"github.com/protobuf-orm/protobuf-orm/graph"
	"example.com/library" // generated package containing File_library_user_proto
)

func main() {
	g := graph.NewGraph()
	if err := graph.Parse(context.Background(), g, library.File_library_user_proto); err != nil {
		panic(err)
	}

	for name, entity := range g.Entities {
		fmt.Println("entity:", name, "key:", entity.Key().Name())
		for f := range entity.Fields() {
			fmt.Printf("  field %s: %s (optional=%t)\n", f.Name(), f.Type(), f.IsOptional())
		}
		for e := range entity.Edges() {
			fmt.Printf("  edge  %s -> %s\n", e.Name(), e.Target().FullName())
		}
	}
}
```

In a real protoc/buf plugin you parse every file at once with
[`graph.ParseFiles`](#public-api):

```go
protogen.Options{}.Run(func(gen *protogen.Plugin) error {
	g := graph.NewGraph()
	if err := graph.ParseFiles(context.Background(), g, gen.Files); err != nil {
		return err
	}
	// walk g.Entities and emit code with gen.NewGeneratedFile(...)
	return nil
})
```

## Defining a schema

### Entities

A message becomes an entity **only** if it carries the `orm.message` option. An
empty option is enough to enable it; `disabled: true` turns it back off.

```proto
message Enabled  { option (orm.message) = {}; }              // entity
message Plain    { /* no option */ }                         // NOT an entity
message Disabled { option (orm.message) = {disabled: true}; } // NOT an entity
```

Every entity must declare exactly one [key](#keys).

### Fields

Mark a field with `orm.field`. If you omit the option entirely (or leave its
`type` unset), the type is **deduced** from the proto field. A field may carry
either `orm.field` or `orm.edge`, never both.

```proto
string name = 5;                                  // deduced: TYPE_STRING
bytes  id   = 1 [(orm.field) = {type: TYPE_UUID, key: true}];
string note = 6 [(orm.field) = {disabled: true}]; // skipped entirely
```

`orm.field` options: `type`, `key`, `unique`, `nullable`, `immutable`,
`default`, `version`, `disabled`.

### Field types

The ORM `Type` is deduced from the proto kind unless you set it explicitly:

| Proto                                  | ORM `Type`            |
| -------------------------------------- | --------------------- |
| `double` / `float`                     | `TYPE_DOUBLE` / `TYPE_FLOAT` |
| `int32`/`sint32`/`sfixed32`            | `TYPE_INT32` / `TYPE_SINT32` / `TYPE_SFIXED32` |
| `int64`/`sint64`/`sfixed64`            | `TYPE_INT64` / `TYPE_SINT64` / `TYPE_SFIXED64` |
| `uint32`/`fixed32`, `uint64`/`fixed64` | `TYPE_UINT32` / `TYPE_FIXED32` / … |
| `bool`, `string`, `bytes`, `enum`      | `TYPE_BOOL` / `TYPE_STRING` / `TYPE_BYTES` / `TYPE_ENUM` |
| `google.protobuf.Timestamp`            | `TYPE_TIME`           |
| `google.protobuf.Struct` / `Value`     | `TYPE_JSON`           |
| any other message                      | `TYPE_JSON`           |
| `map<...>`                             | `TYPE_JSON`           |

Two ORM-specific types are selected explicitly via `type:`:

- `TYPE_UUID` — store `bytes` as a UUID (maps to `github.com/google/uuid.UUID`
  in generated Go).
- `TYPE_JSON` — store a message/map as JSON.

A plain message field that is *not* one of the well-known types must be declared
as JSON (or be an [edge](#edges-relations)); a bare message-typed field is
rejected.

### Nullable & optional

`protobuf-orm` interprets proto **editions presence** into two predicates:

- **`IsNullable()`** — the column may hold `NULL`.
- **`IsOptional()`** — the value need not be supplied when creating a record
  (it is nullable *or* has a default; repeated fields are always optional).

| Field shape                                  | Nullable | Optional |
| -------------------------------------------- | :------: | :------: |
| implicit presence scalar                     |    ✗     |    ✗     |
| explicit presence / `optional` keyword       |    ✓     |    ✓     |
| `(orm.field) = {nullable: true}`             |    ✓     |    ✓     |
| `repeated` / `map`                           |    ✗     |    ✓     |
| has `default`                                |    ✗     |    ✓     |
| `google.protobuf.Timestamp` (presence)       |    ✗*    |    ✗     |

\* a time field is **not** made nullable merely by having message presence; set
`nullable: true` if you want it nullable. Edges follow the same rule: message
presence alone does not make an edge nullable.

### Keys

Exactly one field per entity must be the key:

```proto
bytes id = 1 [(orm.field) = {type: TYPE_UUID, key: true}];
```

The key is implicitly **unique** and **immutable** — you may state those
explicitly, but you cannot contradict them (`key` + `nullable: true`, `key` +
`unique: false`, etc. are errors), and an entity with no key is rejected.

> Composite (multi-field) primary keys are not supported on `main` yet; an
> entity has a single key field.

### Edges (relations)

An edge is a message-typed field annotated with `orm.edge`; its message type
must itself be an entity. Edges model one-to-one and one-to-many relations,
including self-references and cycles.

```proto
message User {
  bytes id = 1 [(orm.field) = {type: TYPE_UUID, key: true}];

  // O2M self-reference: each user has one parent and many children.
  User parent = 10 [(orm.edge) = {}];
  repeated User children = 11 [(orm.edge) = {
    from: {name: "parent", number: 10}   // back-reference to User.parent
  }];

  option (orm.message) = {};
}
```

Edge relationships:

- **`Target()`** — the entity this edge points to.
- **`Inverse()`** — the edge declared by `from:` (here `children.Inverse() ==
  parent`).
- **`Reverse()`** — the opposite edge on the target (here `parent.Reverse() ==
  children`).

Cardinality and uniqueness are inferred: a non-repeated edge on both sides is
treated as one-to-one (and marked unique); a `unique` edge cannot be
`repeated`.

### Indexes

Indexes are declared on the message option and reference props by **name and
number** (both must match):

```proto
option (orm.message) = {
  indexes: [
    {
      name: "child"
      unique: true
      refs: [
        {name: "parent", number: 10},
        {name: "name", number: 5}
      ]
    }
  ]
};
```

An index must reference at least one prop. `hidden: true` writes the index to
the schema (as a unique constraint) but excludes it from generated request
messages.

### Version fields (optimistic locking)

A field marked `version` is the optimistic-locking column. It must be a time
type, there can be at most one per entity, and it cannot be the `key`, nor
`unique`, `nullable`, or `immutable`.

```proto
google.protobuf.Timestamp updated_at = 2 [(orm.field) = {version: {}}];
```

Query it via `entity.HasVersionField()` / `entity.GetVersionField()`.

**The version is the server's to stamp.** A patch may *test* it -- that test is
the lock, and it compiles to a `WHERE` predicate, so the update stays a single
compare-and-swap statement -- but it may not *write* it. `assign` and `remove`
on a version field are both refused with `ormpatch.VersionWriteError`, whether
they arrive in an Apply document or through a PatchRequest's `<name>_force`
carrying a value.

That refusal is what makes the lock mean anything. The version is the token
every client's compare-and-swap is measured against, so a client that could
choose it could re-write the value it had just read: the row would change while
the token stood still, and a concurrent writer's already-stale test would still
hold, and overwrite. The lost update the field exists to prevent would be back,
reported as success.

A `<name>_force` on its own -- with no value -- is still how a caller declines
the lock for its own write, and the server stamps as usual. Declining is legal;
the reason the field is mandatory in a PatchRequest is that an unset field is
the default state of a struct literal, so silence cannot be told apart from
having forgotten. It is not that every write must hold a lock.

**The lock covers updates, not deletes.** `Erase` takes the entity's `Ref` and
nothing else, so there is no way to say "delete only if the row is still at
version V" and no plan to add one: a delete is normally meant absolutely --
delete this user -- where an update is meant relative to what the caller read.
Requiring a precondition on every delete would put a flag on calls that do not
want one, and a flag everyone always sets protects nobody.

The consequence is worth stating plainly, because it is the one place a version
field does not help: a client that read a row, missed a concurrent update to
it, and then erases it, destroys that update and is told the delete succeeded.
Where that matters, the row's own lifecycle has to carry it -- a status column
patched to `deleted` under the lock is a delete this feature does cover.

### RPCs

`orm.message.rpc` generates CRUD service operations. `crud: true` enables all
four; individual operations can be added one at a time or disabled.

```proto
option (orm.message) = {rpc: {crud: true}};                    // Add+Get+Patch+Erase
option (orm.message) = {rpc: {add: {}}};                       // only Add
option (orm.message) = {rpc: {crud: true, add: {disabled: true}}}; // all but Add
```

For an entity `library.User` this yields a `library.UserService` with, e.g.,
`Add(UserAddRequest) -> User`, `Get(UserRef) -> User`, `Patch(UserPatchRequest)
-> google.protobuf.Empty`, and `Erase(UserRef) -> google.protobuf.Empty`.

## Using the graph

```go
g := graph.NewGraph()
_ = graph.Parse(ctx, g, library.File_library_user_proto)

entity := g.Entities["library.User"] // keyed by protoreflect.FullName

entity.Key()                 // the key Field
entity.Fields()              // iter.Seq[Field]
entity.Edges()               // iter.Seq[Edge]
entity.Indexes()             // iter.Seq[Index]
entity.Keys()                // iter.Seq[Elem] — every unique prop/index
entity.Rpcs().HasAdd()       // RPC presence
entity.HasVersionField()     // optimistic locking
```

`Parse` is transactional: it works on a clone and only merges into your graph if
the whole file parses cleanly, so a failed parse never leaves a half-built
entity behind. Successive `Parse` calls accumulate into the same graph, which is
how cross-file relations resolve.

## Public API

`graph` package:

- `NewGraph() *Graph`, `Graph.Clone()`, `Graph.InPlaceMerge(*Graph)`
- `Parse(ctx, *Graph, protoreflect.FileDescriptor) error`
- `ParseFiles(ctx, *Graph, []*protogen.File) error`
- Interfaces: `Entity`, `Prop`, `Field`, `Edge`, `Index`, `Elem`, `Rpc`,
  `RpcMap`, `RpcMessage`, `ProtoTyped`
- Helpers: `GoType`, `GoTypeOf`, `IsCollection`, `GetGoImportPath`,
  `MustGetGoImportPath`, `ProtoType`

`ormpb` package (generated options + helpers):

- Extension handles `E_Message`, `E_Field`, `E_Edge`
- `Type` and its helpers `TypeFromKind`, `DeduceType`, `Type.Decay`,
  `Type.IsScalar`, `Type.IsMessage`
- `Ref` helpers `RefByNumber`, `RefByName`, `Ref.Access`

## Development

This repo uses [buf](https://buf.build) with the opaque protobuf API.

```sh
buf generate     # regenerate ormpb/*.pb.go and internal/examples/**/*.pb.go
go test ./...    # run the test suite
go vet ./...
```

> `internal/examples/**/*.pb.go` is **git-ignored**. After a fresh clone or a
> branch switch, run `buf generate` before `go test`, otherwise you may compile
> against stale generated code from another branch.
