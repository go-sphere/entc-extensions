package main

import (
	"fmt"
	"log"

	"github.com/go-sphere/entc-extensions/entconv"
)

func main() {
	opts := &entconv.Options{
		IDType:            "int64",
		SchemaPath:        "./internal/pkg/database/schema",
		EntPackage:        "github.com/go-sphere/protocgenentconv/testdata/internal/pkg/database/ent",
		ProtoFile:         "./api/entpb/entpb.pb.go",
		ConvPackage:       "converter",        // 生成的 package 名
		ConvPackagePath:   "github.com/go-sphere/protocgenentconv/testdata/api/entpb",
		ProtoAlias:        "entpb",            // 引用原 entpb 包时的别名
		OutDir:            "./internal/converter/",  // 多文件输出目录
	}

	if err := entconv.GenerateConverterFile(opts); err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Printf("Successfully generated converters to %s\n", opts.OutDir)
}
