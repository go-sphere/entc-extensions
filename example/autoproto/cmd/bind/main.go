package main

import (
	"log"

	"github.com/go-sphere/entc-extensions/autoproto/bind"
	"github.com/go-sphere/entc-extensions/autoproto/mapper"
	"github.com/go-sphere/entc-extensions/example/autoproto/api/entpb"
	"github.com/go-sphere/entc-extensions/example/autoproto/ent"
	"github.com/go-sphere/entc-extensions/example/autoproto/ent/example"
)

func main() {
	bindFile := "./render"
	mapperDir := "./mapper"

	if err := createBindFile(bindFile); err != nil {
		log.Fatal(err)
	}
	if err := createMappersFile(mapperDir); err != nil {
		log.Fatal(err)
	}
}

func createMappersFile(dir string) error {
	return mapper.GenerateFiles(&mapper.GenFilesConf{
		Dir: dir,
		Entities: []mapper.GenFileEntityConf{
			{
				Source: ent.Example{},
				Target: entpb.Example{},
			},
			{
				Source: ent.EdgeItem{},
				Target: entpb.EdgeItem{},
			},
		},
	})
}

func createBindFile(dir string) error {
	return bind.GenFiles(&bind.GenFilesConf{
		Dir: dir,
		Entities: []bind.GenFileEntityConf{
			{
				Source:  ent.Example{},
				Target:  entpb.Example{},
				Actions: []any{ent.ExampleCreate{}},
				Options: []bind.GenBindConfOption{
					bind.WithIgnoreFields(example.FieldID),
				},
			},
			{
				Source:  ent.EdgeItem{},
				Target:  entpb.EdgeItem{},
				Actions: []any{ent.EdgeItemCreate{}},
			},
		},
	})
}
