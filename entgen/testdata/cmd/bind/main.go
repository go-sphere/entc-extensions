package main

import (
	"log"

	"github.com/go-sphere/entc-extensions/entgen"
	"github.com/go-sphere/entc-extensions/entgen/conf"
	"github.com/go-sphere/entc-extensions/entgen/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/entgen/testdata/ent"
	"github.com/go-sphere/entc-extensions/entgen/testdata/ent/edgeitem"
	"github.com/go-sphere/entc-extensions/entgen/testdata/ent/example"
	"github.com/go-sphere/entc-extensions/entgen/testdata/mapper"
)

func main() {
	if err := entgen.BindFiles(createFilesConf("./render", "render")); err != nil {
		log.Fatal(err)
	}
}

func createFilesConf(dir, pkg string) *conf.FilesConf {
	return &conf.FilesConf{
		Dir:                  dir,
		Package:              pkg,
		RemoveBeforeGenerate: true,
		Entities: []*conf.EntityConf{
			conf.NewEntity(
				ent.Example{},
				entpb.Example{},
				[]any{ent.ExampleCreate{}},
				conf.WithIgnoreFields(example.FieldID),
				conf.WithCustomFieldConverter(example.FieldEnumValue, mapper.ExampleEnumMap),
			),
			conf.NewEntity(
				ent.EdgeItem{},
				entpb.EdgeItem{},
				[]any{ent.EdgeItemCreate{}},
				conf.WithCustomFieldConverter(edgeitem.FieldEnumValue, mapper.EdgeItemEnumUnmap),
			),
		},
		ExtraImports: nil,
	}
}
