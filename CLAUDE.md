# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **multi-module Go monorepo** that extends [ent](https://entgo.io) ORM with Protocol Buffers (protobuf) support. The repository provides three independent modules that work together to enable a protobuf-first development workflow.

## Modules

| Module | Purpose |
|--------|---------|
| `entproto` | Generates `.proto` files from Ent schemas with auto-annotation support |
| `entconv` | Generates bidirectional converter functions between Ent entities and protobuf messages |
| `entcrud` | Generates CRUD operation code for Ent schemas |

## Common Commands

```bash
# Run all lints across all modules
make lint-all

# Run tests (delegates to testdata)
make test

# Within individual modules:
cd entproto && go test ./...
cd entconv && go test ./...
cd entcrud && go test ./...
```

## Architecture

### entproto
- Works as an `entc.Extension` that hooks into the Ent code generation pipeline
- Generates `.proto` files based on Ent schema definitions
- Key file: `entproto/extension.go` - contains the `Extension` struct and generation hooks
- Supports automatic annotation generation via `WithAutoFill()` option
- Uses `entproto.Message()` and `proto.Field()` annotations to mark schemas/fields for generation

### entconv
- Generates Go code that converts between Ent structs and protobuf structs
- Reads both Ent schema definitions and generated `.pb.go` files
- Key file: `entconv/entconv.go` - contains the `Options` struct and `GenerateConverter` function
- Uses Go templates (see `entconv/internal/generator/template/converter.tmpl`) for code generation

### entcrud
- Generates CRUD service implementations for Ent schemas
- Uses Go AST parsing and code generation
- Key files: `entcrud/bind.go`, `entcrud/internal/bind/func.go`

### testdata
- Integration test/example project demonstrating how all modules work together
- Contains Ent schemas, proto definitions, and generated code
- Can be used as a reference for proper module integration

## Key Dependencies

- `entgo.io/ent` - The Ent ORM
- `github.com/jhump/protoreflect` - Protocol buffer reflection
- `google.golang.org/protobuf` - Protobuf runtime
- `golang.org/x/tools` - Go tools for code generation
