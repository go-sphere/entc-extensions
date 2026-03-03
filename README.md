# entc-extensions

Utilities that streamline using [ent](https://entgo.io) with protobuf. This repository is organized as a **multi-module monorepo** where each top-level directory is a standalone Go module.

## Modules

| Module | Status | Description |
|--------|--------|-------------|
| [`entproto`](./entproto) | **Active** | Fork of the original `entproto`. Strips out RPC/service generation and keeps only the schema-to-protobuf pipeline. Supports automatic annotations and proto3 optional fields. |
| [`entconv`](./entconv) | **Active** | Generates type-safe conversion functions between Ent structs and protobuf structs. |
| [`autoproto`](./autoproto) | **Deprecated** | Functionality merged into `entproto`. |
| [`entgen`](./entgen) | **Deprecated** | Legacy mapper/binder generator. Use `entconv` instead. |

## License

MIT License - see [LICENSE](LICENSE) for details.
