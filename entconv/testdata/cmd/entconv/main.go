package main

import (
	"fmt"
	"log"

	"github.com/go-sphere/entc-extensions/entconv"
)

func main() {
	opts := &entconv.Options{
		IDType:          "int64",
		EntSchema:       "./internal/pkg/database/schema",
		EntImportPath:   "github.com/go-sphere/protocgenentconv/testdata/internal/pkg/database/ent",
		ProtoGoFile:     "./api/entpb/entpb.pb.go",
		ProtoPackage:    "entpb",
		ProtoImportPath: "github.com/go-sphere/protocgenentconv/testdata/api/entpb",
		Output:          "./api/entpb/entpb_entconv.go",
	}

	if err := entconv.GenerateConverterFile(opts); err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Printf("Successfully generated converter to %s\n", opts.Output)
}
