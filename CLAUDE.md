# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **multi-module Go monorepo** that extends [ent](https://entgo.io) ORM with Protocol Buffers (protobuf) support. Each top-level directory (`entproto`, `entconv`, `entcrud`) is an **independent Go module** with its own `go.mod`. `entconv` depends on `entproto` via a `replace` directive pointing at `../entproto`.

## Modules

| Module | Purpose |
|--------|---------|
| `entproto` | Generates `.proto` files from Ent schemas. Fork of `ent/contrib/entproto` with RPC/service generation stripped, native proto3 `optional` support added, and `WithAutoFill()` for automatic annotations. |
| `entconv` | Generates bidirectional `ToProto*`/`ToEnt*` converter functions between Ent entities and protobuf messages. |
| `entcrud` | Generates typed bind helpers between protobuf request structs and Ent mutation builders (e.g. `ent.UserCreate`, `ent.UserUpdateOne`). |
| `testdata` | Integration/example project that drives all three generators end-to-end. Also the place where E2E assertions live. |

## Common Commands

```bash
# Full lint pipeline across all modules (go fix/fmt/vet/get/test, mod tidy,
# golangci-lint fmt+run, nilaway). Use this before committing.
make lint-all

# Regenerate testdata artifacts (proto, ent, conv, crud) and run tests.
# This is what CI runs.
make test

# Regenerate testdata artifacts only (no verification).
make regen

# Verify only — run unit tests per module + testdata E2E assertions.
make verify

# Run a single test in a module:
cd entproto && go test -run TestAdapter ./...
```

The `testdata` module has its own Makefile with `init` (installs `buf` + `protoc-gen-go` at pinned versions), `clean`, `generate`, `verify`, `test`. `make test` at the repo root delegates to `testdata/Makefile generate` then `verify`.

## Module Versioning

Each module is tagged independently using Go's subpath tag convention (`entproto/v0.0.1`, etc.):

```bash
make tag-all TAG=v0.0.1   # creates v0.0.1 + entproto/v0.0.1 + entconv/v0.0.1 + entcrud/v0.0.1
make tag-delete TAG=v0.0.1
```

## Architecture

### Generation pipeline (testdata as reference)

The canonical end-to-end flow lives in `testdata/Makefile`'s `generate` target and runs **in order**:

1. `go run ./cmd/entproto` — runs `entc.Generate` with the `entproto.Extension`, emitting both the Ent code under `internal/pkg/database/ent` and `.proto` files under `proto/entpb`.
2. `buf generate` — compiles `.proto` → `api/entpb/*.pb.go`.
3. `go run ./cmd/entconv` — reads the `.pb.go` AST + Ent schema and emits converter code.
4. `go run ./cmd/entcrud` — emits bind helpers wiring proto request structs to Ent mutation builders.
5. `go mod tidy` — testdata pulls each generator via local module paths.

When debugging a generator change, run only the relevant step from `testdata/` rather than the full `make test`.

### entproto

- Implements `entc.Extension` — hooks into `entc.Generate` rather than running standalone.
- `extension.go` — `Extension` struct + `NewExtension(opts...)`; `WithProtoDir`, `WithAutoFill`, etc.
- `adapter.go` — translates `gen.Type` (Ent's intermediate representation) into `protoreflect`/`descriptorpb` structures.
- `message.go`, `field.go`, `enum.go` — the `entproto.Message()`, `entproto.Field()`, `entproto.Enum()` schema annotations.
- `fix.go` — post-processes generated `.proto` files (e.g. proto3 `optional` keyword handling, which protoreflect doesn't emit natively).
- `WithAutoFill()` injects annotations for schemas/fields that lack them: ID → field 1, others numbered from 2.

### entconv

- Pure code generator (no `entc.Extension` integration).
- Entry point: `entconv.GenerateConverterFile(opts *Options)` in `conv.go`.
- `internal/generator` — parses the `.pb.go` file with `go/ast`/`golang.org/x/tools/go/packages` and walks Ent schema files to align field types.
- `internal/generator/template/converter.tmpl` — the Go template that emits the converter.
- Handles `google.protobuf.Timestamp` ↔ `time.Time`, enum mapping, optional pointer fields.

### entcrud

- Pure code generator, no dependency on `entproto` or `entconv` (only `golang.org/x/tools`).
- Entry point: `entgen.BindFiles(cfg)` in `bind.go`.
- `conf/conf.go` — `FilesConf`, `EntityConf`, options (`WithCustomFieldConverter`, `WithStrictTypeCheck`).
- `internal/inspect` — reflects on `ent.XCreate` / `ent.XUpdateOne` to discover setter methods.
- **Strict type check is on by default** — incompatible field pairs (e.g. `time.Time ← int64`) fail with `*conf.TypeMismatchListError` listing per-field details unless a custom converter is registered via `conf.WithCustomFieldConverter(fieldConst, fn)`.

## Editing Tips

- After modifying a generator, you usually need to run the **full testdata pipeline** (`make regen`) because each generator consumes the previous one's output. Running only the affected module's `go test` will miss integration regressions.
- The `entconv` module pulls `entproto` from `../entproto` via `replace` — when changing `entproto` public API, expect to update both at once.
- `make lint-all` runs `nilaway` with one excluded file (`internal/pkg/database/ent/enttest/enttest.go`); new nil-safety findings should be fixed, not excluded.
