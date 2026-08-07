# Architecture

This document explains how `protobuf-orm` is structured internally and how the
pieces fit together. For usage, see the [README](../README.md).

## Overview

`protobuf-orm` turns annotated Protocol Buffer messages into an in-memory
**schema graph** that downstream code generators can walk to emit ORM code,
database schemas, and gRPC service definitions.

It is **not** a runtime database driver. It is a modeling layer: you feed it
proto [`FileDescriptor`s][filedescriptor] (or `protogen.File`s from a protoc
plugin), and it gives you back a `*graph.Graph` of `Entity` values whose
fields, edges, indexes, keys, and RPCs have been resolved and validated.

```
 .proto files
      │
      │  protoc / buf  (with the orm.* custom options)
      ▼
 FileDescriptor ──────► graph.Parse / graph.ParseFiles ──────► *graph.Graph
 (+ orm extensions)                                              ├─ Entity
                                                                 │   ├─ Field / Edge (Prop)
                                                                 │   ├─ Index
                                                                 │   ├─ Key (Elem)
                                                                 │   └─ Rpcs (RpcMap)
                                                                 └─ ...
```

## Packages

| Package                 | Role                                                                          |
| ----------------------- | ----------------------------------------------------------------------------- |
| `proto/orm/*.proto`     | Source of truth for the custom options (`MessageOptions`, `FieldOptions`, `EdgeOptions`, `Index`, `Ref`, `Order`, `Type`, RPC messages). |
| `ormpb`                 | Generated Go types for those options, the extension handles (`E_Message`, `E_Field`, `E_Edge`), plus hand-written helpers (`type.go`, `ref.go`). |
| `graph`                 | The parser and schema model. Public entry points `Parse`/`ParseFiles`; interfaces `Entity`, `Prop`, `Field`, `Edge`, `Index`, `Rpc`. |
| `internal/iters`        | Small iterator helpers (`Find`).                                              |
| `examples/`             | `.proto` fixtures (also used by tests). `examples/graphtest/invalid/` holds intentionally-broken schemas for failure testing. |
| `internal/examples/`    | Generated Go for the example protos (git-ignored; produced by `buf generate`). |

The `ormpb` and `internal/examples` `*.pb.go` files are generated. The
`internal/examples/**/*.pb.go` files are **git-ignored** and must be produced
locally with `buf generate` — see [Generated code](#generated-code).

## The custom options

Annotations live in `proto/orm/options.proto` and are attached to messages and
fields through proto extensions declared in `proto/orm.proto`:

- `option (orm.message)` → `MessageOptions` — marks a message as an **Entity**
  and configures its `indexes` and `rpc`.
- `[(orm.field) = {...}]` → `FieldOptions` — marks a field as a **Field** prop
  (scalar/JSON/UUID/time data) and sets `key`, `unique`, `nullable`,
  `immutable`, `default`, `type`, `version`, `erased`.
- `[(orm.edge) = {...}]` → `EdgeOptions` — marks a field as an **Edge** prop (a
  relation to another Entity) and sets `bind`, `from`, `unique`, `nullable`,
  `immutable`, `default`.

A field may carry **at most one** of `orm.field` / `orm.edge`. If it carries
neither, it is treated as a field and its `Type` is deduced from the proto kind.

## The model (graph package)

Everything in the graph is built around a few interfaces. Concrete `proto*`
implementations wrap a `protoreflect` descriptor and the parsed option message.

### Entity

`Entity` (`graph/entity.go`) is a parsed message. It exposes its descriptor,
name/package/path, its `Key()`, and iterator sequences over its members:

- `Elems()` — every keyed-or-not member: props followed by indexes.
- `Props()` / `Fields()` / `Edges()` — props, or just the field/edge subset.
- `Indexes()` — declared indexes.
- `Keys()` — every `Elem` that `IsUnique()` (candidate keys).
- `Rpcs()` — the resolved RPC set.

It also answers `Has*()` questions and exposes the optimistic-locking
`GetVersionField()` / `HasVersionField()`.

### Prop, Field, Edge

`Prop` (`graph/prop.go`) is the common interface for a member of an Entity. It
is always *either* a `Field` *or* an `Edge`:

- **Field** (`graph/field.go`) — backed by scalar data. Has an `ormpb.Type`
  (`TYPE_STRING`, `TYPE_UUID`, `TYPE_TIME`, `TYPE_JSON`, …) and can be a
  `version` field for optimistic locking.
- **Edge** (`graph/edge.go`) — backed by a message field that points at another
  Entity. Tracks `Target()`, `Inverse()` (the back-reference edge it declares
  via `from:`) and `Reverse()` (the edge on the target that points back here).

`Prop` exposes presence/constraint predicates: `IsList`, `IsUnique`,
`IsNullable`, `IsImmutable`, `IsOptional`, `HasDefault`.

### Presence semantics

`IsNullable` / `IsOptional` encode the project's interpretation of proto
editions presence, which is subtle:

- **Repeated** props are never nullable (proto cannot distinguish empty from
  null) but are always optional (empty input = empty list/map).
- A **field** is nullable if `nullable:true`, if it uses the `optional`
  keyword, or if it has explicit presence — *except* `TYPE_TIME`, which is not
  nullable merely by having presence.
- An **edge** is nullable only if `nullable:true` or the `optional` keyword is
  used; message presence alone does not make it nullable.
- A prop is **optional** (need not be supplied on create) if it is nullable or
  has a default.

### Index

`Index` (`graph/index.go`) groups one or more props (resolved from `Ref`s by
matching both name and number) and carries `unique` / `immutable` / `hidden`
flags. A `hidden` unique index is written to the schema but excluded from
generated request messages.

### Rpc

`parseRpcs` (`graph/rpc.go`) turns `MessageOptions.rpc` into a `RpcMap` of up to
four operations — `Add`, `Get`, `Patch`, `Erase`. Setting `crud:true` enables
all four; individual operations can then be disabled. Each `Rpc` carries the
synthesized request/response message full-names (e.g. `library.UserAddRequest`,
`library.UserRef`, `google.protobuf.Empty`) under a `<Entity>Service`.

### Type mapping

`graph/type.go` maps a proto field to its Go type (`GoType` / `GoTypeOf`),
handling UUID→`uuid.UUID`, Timestamp→`time.Time`, maps, enums, and nested
message name flattening. `ormpb/type.go` maps between proto kinds and the ORM
`Type` enum (`TypeFromKind`, `DeduceType`) and provides `Decay()` to collapse a
type to its storage category.

## Parsing pipeline

`graph.Parse(ctx, g, fileDescriptor)` (`graph/graph.go`):

1. **Clone** the graph so a failed parse does not mutate the caller's graph
   (`Parse` only merges back on full success).
2. For each top-level message, call `parseEntity`. (Nested messages are not
   parsed as entities.)
3. `parseEntity` (`graph/entity.go`):
   - Skips messages without `orm.message` (or with `disabled:true`).
   - **Forward-declares** the entity in the graph *before* parsing its props, so
     self-references and circular references resolve.
   - Parses each field into a `Field` or `Edge` (`parseProp`). Edge targets that
     are not yet in the graph are parsed recursively.
   - Resolves the **key** (exactly one `key:true` field; made implicitly unique
     and immutable) and validates key constraints.
   - Parses **indexes**.
   - Resolves edge **inverses** via `from:` back-references, inferring O2O vs
     O2M cardinality and unique-ness.
   - Parses **RPCs**.

`graph.ParseFiles(ctx, g, []*protogen.File)` is the protoc-plugin entry point:
it parses every file marked for generation.

## Generated code

Two generation targets, both driven by [`buf.gen.yaml`](../buf.gen.yaml):

- `ormpb/*.pb.go` — generated from `proto/orm/*.proto`. **Committed** to the
  repo.
- `internal/examples/**/*.pb.go` — generated from the `examples/` protos.
  **Git-ignored** (see `.gitignore`); regenerate locally before running tests.

Regenerate everything with:

```sh
buf generate
```

> **Note:** because `internal/examples/**/*.pb.go` is git-ignored, a freshly
> cloned (or branch-switched) checkout has **no** example Go code until you run
> `buf generate`. Stale generated code left over from another branch is a common
> cause of confusing test failures; regenerating fixes it.

The generated code uses the **opaque** protobuf API
(`default_api_level=API_OPAQUE`), which is why option access goes through
`GetX()` / `HasX()` / `SetX()` accessors and `X_builder{...}.Build()`
constructors rather than direct struct fields.

[filedescriptor]: https://pkg.go.dev/google.golang.org/protobuf/reflect/protoreflect#FileDescriptor
