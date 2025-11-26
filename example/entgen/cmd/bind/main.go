package main

import (
	"log"

	"github.com/go-sphere/entc-extensions/entgen"
	"github.com/go-sphere/entc-extensions/entgen/conf"
	"github.com/go-sphere/entc-extensions/example/entgen/api/entpb"
	"github.com/go-sphere/entc-extensions/example/entgen/ent"
	"github.com/go-sphere/entc-extensions/example/entgen/ent/example"
)

func main() {
	if err := entgen.MapperFiles(createFilesConf("./mapper", "mapper")); err != nil {
		log.Fatal(err)
	}
	if err := entgen.BindFiles(createFilesConf("./render", "render")); err != nil {
		log.Fatal(err)
	}
}

func createFilesConf(dir, pkg string) *conf.FilesConf {
	return &conf.FilesConf{
		Dir:                  dir,
		Package:              pkg,
		RemoveBeforeGenerate: false,
		Entities: []*conf.EntityConf{
			conf.NewEntity(
				ent.Example{},
				entpb.Example{},
				[]any{ent.ExampleCreate{}},
				conf.WithIgnoreFields(example.FieldID),
			),
			conf.NewEntity(
				ent.EdgeItem{},
				entpb.EdgeItem{},
				[]any{ent.EdgeItemCreate{}},
			),
		},
		ExtraImports: nil,
	}
}
