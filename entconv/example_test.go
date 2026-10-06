package entconv_test

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/go-sphere/entc-extensions/entconv"
)

func ExampleNewOptions() {
	options := entconv.NewOptions(
		entconv.WithSchemaPath("./internal/pkg/database/schema"),
		entconv.WithEntPackagePath("example.com/project/internal/pkg/database/ent"),
		entconv.WithIDType("int64"),
		entconv.WithProtoFile("./api/entpb/entpb.pb.go"),
		entconv.WithConvPackage("entmap"),
		entconv.WithProtoPackagePath("example.com/project/api/entpb"),
		entconv.WithProtoAlias("entpb"),
		entconv.WithOutDir("./internal/pkg/render/entmap"),
		entconv.WithMissingProtoPolicy(entconv.MissingProtoPolicyWarn),
	)
	fmt.Println(options.EntPackagePath)
	fmt.Println(options.MissingProtoPolicy)
	// Output:
	// example.com/project/internal/pkg/database/ent
	// warn
}

// This example generates converter files for a small fixture schema into a
// temporary directory, the same way an entconv main program would.
func ExampleGenerateConverterFileWithOptions() {
	outDir, err := os.MkdirTemp("", "entmap")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(outDir) }()

	err = entconv.GenerateConverterFileWithOptions(
		entconv.WithSchemaPath("./testdata/fixtures/schema"),
		entconv.WithEntPackagePath("example.com/project/ent"),
		entconv.WithProtoFile("./testdata/fixtures/pb/fixture.pb.go"),
		entconv.WithProtoPackagePath("github.com/go-sphere/entc-extensions/entconv/testdata/fixtures/pb"),
		entconv.WithProtoAlias("pb"),
		entconv.WithOutDir(outDir),
	)
	if err != nil {
		log.Fatal(err)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range entries {
		fmt.Println(entry.Name())
	}
	// Output:
	// post.go
	// user.go
}

// This example shows how to tell an option error apart from other failures.
func ExampleGenerateConverter_requiredOption() {
	_, err := entconv.GenerateConverterWithOptions(
		entconv.WithProtoFile(""),
	)
	var required *entconv.RequiredOptionError
	if errors.As(err, &required) {
		fmt.Println("missing option:", required.Field)
	}
	// Output: missing option: ProtoFile
}
