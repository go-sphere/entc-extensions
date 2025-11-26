# entc-extensions

Utilities that streamline using [ent](https://entgo.io) with protobuf backends. The repository currently hosts the `autoproto` extension plus helper generators for mapper/binder code.

## Packages

- `autoproto`: augments `entc` runs to emit `entproto` annotations automatically, render `.proto` descriptors, and scaffold Go mappers/binders between Ent models and generated protobuf structs.

## autoproto

`autoproto` wraps the standard `entproto` pipeline so you get generated protobuf definitions without manually decorating every schema. It also ships Go code generators (under `entgen`) to keep Ent models and protobuf stubs in sync.

### Installation

```bash
go get github.com/go-sphere/entc-extensions/autoproto
```

### Use the entc extension

1. Add the extension to your `entc.Generate` invocation:

   ```go
   err := entc.Generate("./schema", &gen.Config{
   	Target: "./ent",
   	// other ent options...
   }, entc.Extensions(autoproto.NewAutoProtoExtension(&autoproto.ProtoOptions{
   	Graph:    autoproto.NewDefaultGraphOptions(),
   	ProtoDir: "./proto",
   })))
   ```

2. Run your codegen (see `example/autoproto/cmd/ent`). The extension fixes schema annotations, applies the type/enum/optional rules in `GraphOptions`, and writes compact `.proto` files into `ProtoDir`.

3. Use `buf` or `protoc` to generate Go stubs from the emitted `.proto` files, then run the mapper/binder generators (see below) to keep your Ent models and protobuf API in sync.

### Graph options

`ProtoOptions.Graph` tunes how the Ent graph is rewritten before `entproto` renders descriptors:

| Option                 | Default                                          | Description                                                                                                                                                           |
|------------------------|--------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `AllFieldsRequired`    | `true`                                           | When `true`, all optional fields/edges are forced to required; when `false`, proto3 optional markers are injected so the generated `.proto` keeps optional semantics. |
| `AutoAddAnnotation`    | `true`                                           | Auto-injects `entproto.Message`/`entproto.Field` annotations and assigns field numbers, respecting any numbers you already set.                                       |
| `EnumUseRawType`       | `true`                                           | When `true`, enums are rendered as their raw Go type (string/int); set to `false` to emit protobuf enum definitions via `entproto.Enum`.                              |
| `SkipUnsupported`      | `true`                                           | Unsupported JSON/Other fields get `entproto.Skip`; set to `false` to map them to `UnsupportedProtoType`.                                                              |
| `TimeProtoType`        | `"int64"`                                        | Converts `field.Time` to `int64` or `string`.                                                                                                                         |
| `UUIDProtoType`        | `"string"`                                       | Converts `field.UUID` to a protobuf string.                                                                                                                           |
| `UnsupportedProtoType` | `"google.protobuf.Any"`                          | Message/type used when `SkipUnsupported` is `false` (use `"bytes"` to emit a bytes field).                                                                            |
| `ProtoPackages`        | `google/protobuf/any.proto,google.protobuf,Any;` | Extra proto imports to register with `entproto`; use `ParseProtoPackages` to build the slice.                                                                         |

`LoadGraph`/`FixGraph` expose the same transformations if you need to adjust a loaded `gen.Graph` outside of the extension flow.

### Mapper & bind generators

Generators live under `entgen` and take a shared `FilesConf` (`Dir`, `Package`, `RemoveBeforeGenerate`, `ExtraImports`, `Entities`). Use `conf.NewEntity` to describe a source/target pair and optional ent mutation actions for binders.

```go
files := &conf.FilesConf{
	Dir:     "./mapper",
	Package: "mapper",
	Entities: []*conf.EntityConf{
		conf.NewEntity(
			ent.Example{},
			entpb.Example{},
			nil,
			conf.WithIgnoreFields(example.FieldID),
		),
	},
}
if err := entgen.MapperFiles(files); err != nil {
	log.Fatal(err)
}

bindFiles := &conf.FilesConf{
	Dir:     "./render",
	Package: "render",
	Entities: []*conf.EntityConf{
		conf.NewEntity(
			ent.Example{},
			entpb.Example{},
			[]any{ent.ExampleCreate{}}, // ent mutations to generate bind helpers for
			conf.WithIgnoreFields(example.FieldID),
		),
	},
}
if err := entgen.BindFiles(bindFiles); err != nil {
	log.Fatal(err)
}
```

Mapper output provides `ToProto{Entity}` and `ToProto{Entity}List` functions that map exported fields with matching (snake-cased) names and allow per-call modifiers for special cases. Bind output creates one function per ent mutation (e.g. `CreateExample`) plus an `options.go` helper file.

Generated binders accept `options ...Option` at call sites:

- `IgnoreField(...)` / `KeepFieldsOnly(...)` – skip or explicitly allow fields (use ent field constants, e.g. `example.FieldName`).
- `IgnoreSetZeroField(...)` – only set a field when the protobuf value is non-zero.
- `ClearOnNilField(...)` – when the protobuf field is `nil`, call the ent mutation’s `Clear<Field>` setter if available.

Extra imports can be supplied as `[2]string{path, alias}` entries; aliases are optional and deduplicated automatically. `RemoveBeforeGenerate` wipes the output directory before generation if you want a clean slate.

### Example project

`example/autoproto` demonstrates the full pipeline:

- `make run` cleans previous artifacts, runs `entc` with the extension, generates protobuf code via `buf`, and regenerates mapper/bind helpers.
- `cmd/ent` configures the extension; `cmd/bind` wires the generators.
- Generated assets end up in `example/autoproto/{ent,proto,api,mapper,render}`.

## License

**entc-extensions** is released under the MIT license. See [LICENSE](LICENSE) for details.
