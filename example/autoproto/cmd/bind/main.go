package main

import (
	"log"

	"github.com/go-sphere/entc-extensions/autoproto/gen"
	"github.com/go-sphere/entc-extensions/autoproto/gen/conf"
	"github.com/go-sphere/entc-extensions/example/autoproto/api/entpb"
	"github.com/go-sphere/entc-extensions/example/autoproto/ent"
	"github.com/go-sphere/entc-extensions/example/autoproto/ent/example"
)

func main() {
	if err := gen.MapperFiles(createFilesConf("./mapper", "mapper")); err != nil {
		log.Fatal(err)
	}
	if err := gen.BindFiles(createFilesConf("./render", "render")); err != nil {
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
