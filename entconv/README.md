# entconv

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A Go library that automatically generates bidirectional converter code between [Ent](https://entgo.io) (ORM) and [Protocol Buffers](https://protobuf.dev) types.

## Overview

When building gRPC services with Ent ORM, you often need to convert between your Ent entities and Protobuf messages. Writing these converters manually is tedious and error-prone. `entconv` automates this process by generating type-safe conversion functions based on your Ent schema and generated Protobuf Go code.

### Features

- **Bidirectional Conversion**: Generate `ToProto` and `ToEnt` functions for each entity
- **Type-Safe**: Leverages Go's type system for compile-time safety
- **EntProto Compatible**: Works seamlessly with `entproto` annotations
- **Configurable**: Flexible options to match your project structure
- **Zero Dependencies**: Generated code has minimal external dependencies
- **Enum Support**: Automatic conversion between Ent enums and Protobuf enums
- **Time Support**: Converts Ent `time.Time` values to and from protobuf `int64` Unix seconds
- **Presence Support**: Preserves nil versus zero values for Ent `Optional().Nillable()` scalar fields generated as proto3 `optional`

## Installation

```bash
go get github.com/go-sphere/entc-extensions/entconv
```

## Quick Start

### 1. Define Your Ent Schema

Annotate your Ent schema with `entproto` annotations:

```go
package schema

import (
    "github.com/go-sphere/entc-extensions/entproto"
    "entgo.io/ent"
    "entgo.io/ent/schema"
    "entgo.io/ent/schema/field"
)

type User struct {
    ent.Schema
}

func (User) Annotations() []schema.Annotation {
    return []schema.Annotation{
        entproto.Message(),
    }
}

func (User) Fields() []ent.Field {
    return []ent.Field{
        field.String("name").Annotations(entproto.Field(2)),
        field.Int("age").Annotations(entproto.Field(3)),
        field.Bool("active").Annotations(entproto.Field(4)),
    }
}
```

### 2. Generate Protobuf Definitions

Use `entproto` to generate `.proto` files and compile them:

```bash
# Run the local Ent generator program, which installs the entproto extension
go run ./cmd/entproto

# Compile proto files (using buf or protoc)
buf generate
```

### 3. Generate Converters

Create a generator program or use the library directly:

```go
package main

import (
	"log"

	"github.com/go-sphere/entc-extensions/entconv"
)

func main() {
	opts := &entconv.Options{
		SchemaPath:       "./internal/pkg/database/schema",
		EntPackagePath:   "github.com/example/project/internal/pkg/database/ent",
		IDType:            "int64",
		ProtoFile:         "./api/entpb/entpb.pb.go",
		ConvPackage:       "entmap",
		ProtoPackagePath: "github.com/example/project/api/entpb",
		ProtoAlias:       "entpb",
		OutDir:           "./internal/pkg/render/entmap",
	}

	if err := entconv.GenerateConverterFile(opts); err != nil {
		log.Fatalf("Failed to generate converter: %v", err)
	}
}
```

### 4. Use Generated Converters

```go
package main

import (
	"log"

	"github.com/example/project/internal/pkg/database/ent"
	"github.com/example/project/internal/pkg/render/entmap"
)

func main() {
	// Ent entity from database
	user := &ent.User{
		ID:   1,
		Name: "John Doe",
		Age:  30,
	}

	// Convert to Protobuf message
	pbUser, err := entmap.ToProtoUser(user)
	if err != nil {
		log.Fatal(err)
	}

	// Convert back to Ent entity
	entUser, err := entmap.ToEntUser(pbUser)
	if err != nil {
		log.Fatal(err)
	}
	_ = entUser
}
```

## Configuration Options

| Option | Required | Description | Default |
|--------|----------|-------------|---------|
| `SchemaPath` | Yes | Directory containing Ent schema definitions | `./internal/pkg/database/schema` |
| `EntPackagePath` | Yes | Import path for the generated Ent package | Current module + `/internal/pkg/database/ent` |
| `IDType` | No | ID type for Ent schema: `int`, `int64`, `uint`, `uint64`, `string` | `int64` |
| `ProtoFile` | Yes | Path to the generated `.pb.go` file | `./api/entpb/entpb.pb.go` |
| `ConvPackage` | Yes | Package name for generated converters | `entmap` |
| `ProtoPackagePath` | Yes | Import path for the protobuf Go package | Current module + `/api/entpb` |
| `ProtoAlias` | Yes | Import alias used for protobuf types | `entpb` |
| `OutDir` | Yes | Output directory for generated converter files | `./internal/pkg/render/entmap` |
| `MissingProtoPolicy` | No | `strict` returns an error; `warn` generates matched types and invokes `WarningHandler` | `strict` |

## Supported Type Mappings

| Ent Type | Protobuf Type | Notes |
|----------|--------------|-------|
| `string` | `string` | Direct mapping |
| `int` | `int32` / `int64` | Configurable |
| `int64` | `int64` | Direct mapping |
| `uint` | `uint32` / `uint64` | Configurable |
| `uint64` | `uint64` | Direct mapping |
| `bool` | `bool` | Direct mapping |
| `float64` | `double` | Direct mapping |
| `time.Time` | `int64` | Unix seconds |
| `[]byte` | `bytes` | Direct mapping |
| Enum | Enum | Automatic conversion |
| Optional nillable scalar | proto3 `optional` scalar | Nil/presence is preserved |

## Generated Code Example

Given a `User` entity, the following functions are generated:

```go
// ToProtoUser converts an ent.User to a proto User message
func ToProtoUser(e *ent.User) (*entpb.User, error)

// ToEntUser converts a proto User message to an ent.User
func ToEntUser(v *entpb.User) (*ent.User, error)

// Enum conversion (if applicable)
func ToProtoUserStatus(e user.Status) entpb.User_Status
func ToEntUserStatus(e entpb.User_Status) user.Status
```

## Project Structure Example

```
project/
├── api/
│   └── entpb/
│       └── entpb.pb.go       # Generated by protoc
├── proto/
│   └── entpb/
│       └── entpb.proto       # Generated by entproto
├── internal/
│   └── pkg/
│       ├── database/
│       │   ├── ent/          # Generated Ent code
│       │   └── schema/       # Ent schema definitions
│       └── render/
│           └── entmap/       # Generated by entconv
└── cmd/
    └── genconv/
        └── main.go           # Converter generator
```

## Requirements

- Go 1.23 or later
- Ent v0.14.0 or later
- entproto (for proto generation)

## Testing

Run the test suite:

```bash
go test ./...
```

Run tests with the testdata example:

```bash
make test
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

- [Ent](https://entgo.io) - The excellent Go ORM framework
- [entproto](https://github.com/ent/contrib/tree/master/entproto) - Protobuf support for Ent
