# entc-extensions

Utilities that streamline using [ent](https://entgo.io) with protobuf. This repository is organized as a **multi-module monorepo** where each top-level directory is a standalone Go module.

## Modules

| Module                   | Description                                                                                                                                                                   |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [`entproto`](./entproto) | Fork of the original `entproto`. Strips out RPC/service generation and keeps only the schema-to-protobuf pipeline. Supports automatic annotations and proto3 optional fields. |
| [`entconv`](./entconv)   | Generates type-safe conversion functions between Ent structs and protobuf structs.                                                                                            |
| [`entcrud`](./entcrud)   | Specialized code generator for CRUD operations on Ent schemas.                                                                                                                |

## License

MIT License - see [LICENSE](LICENSE) for details.