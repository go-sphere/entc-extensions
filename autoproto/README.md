# autoproto

**⚠️ DEPRECATED: This module is no longer maintained.**

`autoproto` was an `entc` extension that automatically injected `entproto` annotations into Ent schemas. This functionality has been merged into [`entproto`](../entproto).

## Migration

Replace `autoproto.NewAutoProtoExtension` with `entproto.NewExtension`, and use `entproto.WithAutoFill()` to enable automatic annotation generation:

```go
// Before (autoproto)
import "github.com/go-sphere/entc-extensions/autoproto"

err := entc.Generate("./schema", &gen.Config{}, 
    entc.Extensions(autoproto.NewAutoProtoExtension(&autoproto.ProtoOptions{
        ProtoDir: "./proto",
    })),
)

// After (entproto)
import "github.com/go-sphere/entc-extensions/entproto"

ext, _ := entproto.NewExtension(
    entproto.WithProtoDir("./proto"),
    entproto.WithAutoFill(), // Enable automatic annotation generation
)

err := entc.Generate("./schema", &gen.Config{}, 
    entc.Extensions(ext),
)
```

**Key migration points:**
- Use `entproto.WithAutoFill()` to keep the auto-annotation behavior from `autoproto`
- `entproto.NewExtension()` returns `(*Extension, error)` instead of a single value
- Options are passed as functional options instead of a config struct

See [`entproto/README.md`](../entproto/README.md) for full documentation.
