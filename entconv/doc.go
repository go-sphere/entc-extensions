// Package entconv generates Go functions that convert between ent entities and
// the protobuf messages produced from them by
// [github.com/go-sphere/entc-extensions/entproto].
//
// For every ent schema T that has a message of the same name in the compiled
// .pb.go file, the generated code provides ToProtoT(*ent.T) (*pb.T, error) and
// ToEntT(*pb.T) (*ent.T, error), plus per-enum helpers named like
// ToProtoT_Field and ToEntT_Field. time.Time fields are converted to and from
// int64 Unix seconds, and nillable fields to and from proto3 optional fields.
//
// entconv is a plain code generator, not an entc extension. Run it from its own
// program after the ent code and the .pb.go file exist, typically as a step
// after entc + protoc/buf generation. The recommended entry point is
// [GenerateConverterFileWithOptions], which writes one file per entity;
// [GenerateConverter] returns the source in memory instead.
//
// # Usage
//
// With the scaffold layout assumed by [DefaultOptions] (schema in
// ./internal/pkg/database/schema, ent package in internal/pkg/database/ent,
// generated protobuf in ./api/entpb/entpb.pb.go), run from the module root:
//
//	import (
//		"log"
//
//		"github.com/go-sphere/entc-extensions/entconv"
//	)
//
//	func main() {
//		if err := entconv.GenerateConverterFileWithOptions(); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// For any other layout, override the defaults with the With* options:
//
//	err := entconv.GenerateConverterFileWithOptions(
//		entconv.WithSchemaPath("./ent/schema"),
//		entconv.WithEntPackagePath("example.com/app/ent"),
//		entconv.WithProtoFile("./gen/entpb/entpb.pb.go"),
//		entconv.WithProtoPackagePath("example.com/app/gen/entpb"),
//		entconv.WithOutDir("./internal/entmap"),
//	)
//
// # Requirements
//
//   - The schema must carry the same entproto annotations used to generate the
//     .proto file; descriptors are rebuilt with entproto.LoadAdapter.
//   - IDType must match the gen.Config.IDType used for ent code generation.
//   - The .pb.go file is parsed without type checking, so it must be the file
//     that declares the message structs and enum constants.
//
// Generation fails before writing anything when options are invalid, when ent
// schemas have no message (under [MissingProtoPolicyStrict]), or when the .pb.go
// contents do not match what the schema implies; see [GenerateConverter] for
// the error types.
//
// # Related packages
//
// [github.com/go-sphere/entc-extensions/entcrud] generates helpers that copy
// protobuf request messages into ent create/update builders instead of
// converting whole entities.
package entconv
