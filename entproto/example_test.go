package entproto_test

import (
	"fmt"
	"log"

	"entgo.io/ent"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// This example wires the extension into an entc program. It is compile-only
// because entc.Generate writes the ent package and .proto files to disk.
func ExampleNewExtension() {
	ext, err := entproto.NewExtension(
		entproto.WithProtoDir("./proto"),
		entproto.WithAutoFill(),
	)
	if err != nil {
		log.Fatal(err)
	}
	err = entc.Generate("./ent/schema", &gen.Config{
		Target:  "./ent",
		Package: "example.com/app/ent",
	}, entc.Extensions(ext))
	if err != nil {
		log.Fatal("running ent codegen: ", err)
	}
}

// This example shows the annotations a schema's Fields method returns when
// WithAutoFill is not used.
func ExampleField() {
	fields := []ent.Field{
		field.String("name").
			Annotations(entproto.Field(2)),
		field.String("nickname").
			Optional().
			Nillable(). // emitted as a proto3 optional field
			Annotations(entproto.Field(3)),
		field.Enum("status").
			Values("active", "banned").
			Annotations(
				entproto.Field(4),
				entproto.Enum(map[string]int32{"active": 1, "banned": 2}),
			),
		field.JSON("created", &timestamppb.Timestamp{}).
			Annotations(entproto.MessageField(5, &timestamppb.Timestamp{})),
		field.String("password_hash").
			Annotations(entproto.Skip()),
	}
	fmt.Println(len(fields))
	// Output: 5
}

// This example builds descriptors from a schema directory without writing
// files, and inspects how protobuf fields map back to the ent schema.
func ExampleLoadAdapter() {
	graph, err := entc.LoadGraph("./testdata/schema/multipkg", &gen.Config{
		Package: "example.com/app/ent",
	})
	if err != nil {
		log.Fatal(err)
	}
	adapter, err := entproto.LoadAdapter(graph)
	if err != nil {
		log.Fatal(err)
	}
	fd, err := adapter.GetFileDescriptor("Group")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(fd.GetName(), fd.GetPackage())

	fields, err := adapter.FieldMap("Group")
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range fields.Fields() {
		fmt.Println("field:", f.PbStructField())
	}
	for _, e := range fields.Edges() {
		fmt.Println("edge:", e.PbStructField(), "->", e.ReferencedPbType.GetFullyQualifiedName())
	}
	// Output:
	// acme/group/v1/v1.proto acme.group.v1
	// field: Id
	// field: Name
	// edge: Members -> acme.user.v1.User
	// edge: Owner -> acme.user.v1.User
}

func ExampleNormalizeEnumIdentifier() {
	fmt.Println(entproto.NormalizeEnumIdentifier("in-progress"))
	fmt.Println(entproto.NormalizeEnumIdentifier("api v2"))
	// Output:
	// IN_PROGRESS
	// API_V2
}
