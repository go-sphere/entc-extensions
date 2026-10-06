// Package entproto generates proto3 .proto files from an ent schema during
// ent code generation.
//
// It is a fork of entgo.io/contrib/entproto with RPC/service generation
// removed, native proto3 optional fields for nillable ent fields, and
// [WithAutoFill] for schemas that carry no entproto annotations. Only message
// and enum definitions are emitted.
//
// The recommended entry point is [NewExtension], registered with
// [entgo.io/ent/entc.Extensions] in the program that calls
// [entgo.io/ent/entc.Generate] (usually ent/entc.go). The extension runs after
// ent has written its own code, then writes one .proto file per protobuf
// package.
//
// # Usage
//
//	import (
//		"log"
//
//		"entgo.io/ent/entc"
//		"entgo.io/ent/entc/gen"
//		"github.com/go-sphere/entc-extensions/entproto"
//	)
//
//	func main() {
//		ext, err := entproto.NewExtension(
//			entproto.WithProtoDir("./proto"), // default: <gen.Config.Target>/proto
//			entproto.WithAutoFill(),          // optional: annotate everything automatically
//		)
//		if err != nil {
//			log.Fatal(err)
//		}
//		err = entc.Generate("./ent/schema", &gen.Config{
//			Target:  "./ent",
//			Package: "example.com/app/ent",
//		}, entc.Extensions(ext))
//		if err != nil {
//			log.Fatal(err)
//		}
//	}
//
// Without [WithAutoFill], annotate each schema that should become a message and
// give each of its fields and edges a [Field] number or a [Skip] annotation:
//
//	func (User) Annotations() []schema.Annotation {
//		return []schema.Annotation{entproto.Message()} // proto package "entpb"
//	}
//
//	func (User) Fields() []ent.Field {
//		return []ent.Field{
//			field.String("name").Annotations(entproto.Field(2)),
//			field.Enum("status").Values("active", "banned").Annotations(
//				entproto.Field(3),
//				entproto.Enum(map[string]int32{"active": 1, "banned": 2}),
//			),
//			field.String("secret").Annotations(entproto.Skip()),
//		}
//	}
//
// Field number 1 is reserved for the ID field; the default ent ID receives it
// implicitly.
//
// # Output layout
//
// A protobuf package a.b.c is written to a/b/c/c.proto under the proto
// directory; the default package "entpb" therefore produces entpb/entpb.proto.
// The go_package option is set to <gen.Config.Package>/proto/<package with dots
// replaced by slashes> and is not configurable; use buf managed mode or a
// protoc go_package override when a different Go import path is needed.
//
// # Related packages
//
// [github.com/go-sphere/entc-extensions/entconv] generates ent <-> protobuf
// converter functions from the compiled .pb.go output, and
// [github.com/go-sphere/entc-extensions/entcrud] generates helpers that copy
// protobuf request messages into ent create/update builders. For lower-level
// access to the descriptors without writing files, use [LoadAdapter].
package entproto
