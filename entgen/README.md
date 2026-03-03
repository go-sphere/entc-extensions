# entgen

**⚠️ DEPRECATED: This module is no longer maintained.**

`entgen` was a code generator for mapper and binder utilities between Ent models and protobuf structs. It required manual registration with Ent mutations.

## Migration

Use [`entconv`](../entconv) instead. It provides:
- Simpler API with no manual mutation binding
- Better type safety
- Automatic bidirectional conversion

See [`entconv/README.md`](../entconv/README.md) for documentation and usage examples.
