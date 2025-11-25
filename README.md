# entc-extensions

Utilities that streamline using [ent](https://entgo.io) with protobuf backends. The repository currently hosts the `autoproto` extension plus helper generators for mapper/binder code.

## Packages

- `autoproto`: augments `entc` runs to emit `entproto` annotations automatically, render `.proto` descriptors, and scaffold Go mappers/binders between Ent models and generated protobuf structs.

## autoproto

`autoproto` wraps the standard `entproto` pipeline so you get generated protobuf definitions without manually decorating every schema. It consists of three cooperating parts:

- **Extension** – `autoproto.NewAutoProtoExtension` fixes the in-memory `entc` graph, injects sensible defaults (field numbering, proto3 optional handling, enum conversions), and prints `.proto` files via `entproto` once the Ent graph is generated.
- **Mapper generator** – `autoproto/mapper` produces one file per entity that converts between Ent structs and the Go types produced by `protoc`/`buf`.
- **Bind generator** – `autoproto/bind` emits helpers that populate Ent mutations (`*Create`, `*Update`, etc.) from protobuf messages.

### Installation

```bash
go get github.com/go-sphere/entc-extensions/autoproto
```

### Quick start

1. Add the extension to your `entc.Generate` invocation:

   ```go
   err := entc.Generate("./schema", &gen.Config{ /* ... */ },
	   entc.Extensions(autoproto.NewAutoProtoExtension(&autoproto.ProtoOptions{
		   Graph:    autoproto.NewDefaultGraphOptions(),
		   ProtoDir: "./proto",
	   })))
   ```

2. Run your codegen (see `example/autoproto/cmd/ent`). The extension fixes schema annotations and writes `.proto` files into `ProtoDir` (defaults to `ent/<pkg>/proto`).

3. Use `buf` or `protoc` to generate Go stubs from the emitted `.proto` files, then run the mapper/binder generators similar to `example/autoproto/cmd/bind` to keep your Ent models and protobuf API in sync.

### Graph options

`ProtoOptions.Graph` accepts fine-grained knobs to align Ent types with protobuf expectations:

| Option | Default | Description |
| --- | --- | --- |
| `AllFieldsRequired` | `true` | Force fields/edges to be non-optional unless annotated; when `false`, `FieldIsProto3Optional` markers are added so generated protos keep optional semantics. |
| `AutoAddAnnotation` | `true` | Automatically inject `entproto.Message` and `entproto.Field` annotations so your schemas stay clean. |
| `EnumUseRawType` | `true` | When set, enum fields fall back to their raw Go type (string/int) instead of emitting a protobuf enum definition. |
| `SkipUnsupported` | `true` | Unsupported field types get an `entproto.Skip` annotation; when `false`, they map to `UnsupportedProtoType`. |
| `TimeProtoType` | `"int64"` | Converts `field.Time` to `int64` or `string`. |
| `UUIDProtoType` | `"string"` | Converts `field.UUID` into a protobuf string field. |
| `UnsupportedProtoType` | `"google.protobuf.Any"` | Target message used when `SkipUnsupported` is disabled. |
| `ProtoPackages` | `google/protobuf/any.proto,google.protobuf,Any;` | Extra well-known-type style imports to register before printing files. |

### Mapper & bind helpers

Both helpers follow the same pattern: describe source/target pairs and let the generator write formatted Go files.

```go
// Mapper — converts between ent.{Entity} and entpb.{Entity}.
err := mapper.GenerateFiles(&mapper.GenFilesConf{
	Dir: "./mapper",
	Entities: []mapper.GenFileEntityConf{{
		Source: ent.Example{},
		Target: entpb.Example{},
	}},
})

// Bind — populates ent mutations from protobuf payloads.
err := bind.GenFiles(&bind.GenFilesConf{
	Dir: "./render",
	Entities: []bind.GenFileEntityConf{{
		Source:  ent.Example{},
		Target:  entpb.Example{},
		Actions: []any{ent.ExampleCreate{}},
	}},
})
```

See `example/autoproto/cmd/bind` for a more complete setup that regenerates binding helpers alongside mappers after running `buf generate`.

### Example project

The `example/autoproto` folder demonstrates the full workflow:

- `make run` cleans previous artifacts, runs `entc`, generates protobuf code via `buf`, and regenerates the mapper/binder helpers.
- `cmd/ent` shows how to configure the extension, while `cmd/bind` orchestrates the mapper/bind generators.
- Generated assets end up in `example/autoproto/{ent,proto,api,mapper,render}`.

## License

**entc-extensions** is released under the MIT license. See [LICENSE](LICENSE) for details.
