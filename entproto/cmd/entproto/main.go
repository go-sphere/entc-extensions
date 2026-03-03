package main

import (
	"flag"
	"log"
	"path/filepath"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"github.com/go-sphere/entc-extensions/entproto"
)

func main() {
	var (
		schemaPath = flag.String("path", "", "path to schema directory")
	)
	flag.Parse()
	if *schemaPath == "" {
		log.Fatal("entproto: must specify schema path. use entproto -path ./ent/schema")
	}
	abs, err := filepath.Abs(*schemaPath)
	if err != nil {
		log.Fatalf("entproto: failed getting absolute path: %v", err)
	}
	graph, err := entc.LoadGraph(*schemaPath, &gen.Config{
		Target: filepath.Dir(abs),
	})
	if err != nil {
		log.Fatalf("entproto: failed loading ent graph: %v", err)
	}
	if err := entproto.Generate(graph); err != nil {
		log.Fatalf("entproto: failed generating protos: %s", err)
	}
}
