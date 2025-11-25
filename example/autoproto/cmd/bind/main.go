package main

import (
	"log"

	"github.com/go-sphere/entc-extensions/autoproto/bind"
	"github.com/go-sphere/entc-extensions/autoproto/mapper"
	"github.com/go-sphere/entc-extensions/autoproto/utils/inspect"
	"github.com/go-sphere/entc-extensions/example/autoproto/api/entpb"
	"github.com/go-sphere/entc-extensions/example/autoproto/ent"
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
			{Source: ent.Example{}, Target: entpb.Example{}},
			{Source: ent.EdgeItem{}, Target: entpb.EdgeItem{}},
		},
	})
}

func createBindFile(dir string) error {
	return bind.GenFiles(&bind.GenFilesConf{
		Dir: dir,
		Entities: []bind.GenFileEntityConf{
			{
				Name:    inspect.TypeName(entpb.Example{}),
				Actions: []any{ent.ExampleCreate{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.Example{}, entpb.Example{}, act)
				},
			},
			{
				Name:    inspect.TypeName(entpb.EdgeItem{}),
				Actions: []any{ent.EdgeItemCreate{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.EdgeItem{}, entpb.EdgeItem{}, act)
				},
			},
		},
	})
}
