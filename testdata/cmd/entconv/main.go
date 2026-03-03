package main

import (
	"log"
	"path"
	"runtime/debug"

	"github.com/go-sphere/entc-extensions/entconv"
)

func main() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	opts := &entconv.Options{
		IDType:           "int64",
		SchemaPath:       "./internal/pkg/database/schema",
		EntPackagePath:   path.Join(info.Main.Path, "/internal/pkg/database/ent"),
		ProtoFile:        "./api/entpb/entpb.pb.go",
		ConvPackage:      "entmap",
		ProtoPackagePath: path.Join(info.Main.Path, "/api/entpb"),
		ProtoAlias:       "entpb",
		OutDir:           "./internal/pkg/render/entmap",
	}
	if err := entconv.GenerateConverterFile(opts); err != nil {
		log.Fatalf("error: %v", err)
	}
}
