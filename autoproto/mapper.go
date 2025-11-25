package autoproto

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"

	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc/gen"
	"github.com/go-sphere/entc-extensions/autoproto/mapper"
	"golang.org/x/tools/imports"
)

type MapperOptions struct {
	Graph         *Options
	MapperDir     string
	MapperPackage string
	EntPackage    string
	ProtoPkgPath  string
	ProtoPkgName  string
}

func GenerateMapper(options *MapperOptions) error {
	gh, err := LoadGraph(options.Graph)
	if err != nil {
		return err
	}
	err = generateMappers(gh, options)
	if err != nil {
		return err
	}
	return nil
}

func generateMappers(graph *gen.Graph, options *MapperOptions) error {
	adapter, err := entproto.LoadAdapter(graph)
	if err != nil {
		return fmt.Errorf("entproto: failed loading adapter: %w", err)
	}
	_ = os.RemoveAll(options.MapperDir)
	err = os.MkdirAll(options.MapperDir, 0755)
	if err != nil {
		return fmt.Errorf("entproto: failed creating entmapper dir: %w", err)
	}
	for _, node := range graph.Nodes {
		msgDesc, nErr := adapter.GetMessageDescriptor(node.Name)
		if nErr != nil {
			continue
		}
		fileDesc := msgDesc.GetFile()
		g, nErr := mapper.NewGenerator(fileDesc, graph, adapter, node, msgDesc)
		if nErr != nil {
			return nErr
		}

		if options.EntPackage != "" {
			g.EntImportPath = mapper.GoImportPath(options.EntPackage)
		}
		if options.ProtoPkgPath != "" {
			g.ProtoImportPath = mapper.GoImportPath(options.ProtoPkgPath)
		}
		if options.MapperPackage != "" {
			g.PackageName = options.MapperPackage
		}
		if options.ProtoPkgName != "" {
			g.ProtoPackageName = options.ProtoPkgName
		}
		content, nErr := g.Generate()
		if nErr != nil {
			return nErr
		}
		fileName := gen.Funcs["snake"].(func(string) string)(node.Name) + ".go"
		formatted, fmtErr := format.Source(content)
		if fmtErr != nil {
			return fmt.Errorf("entproto: format entmapper for %s: %w", node.Name, fmtErr)
		}
		fixImport, nErr := imports.Process(fileName, formatted, nil)
		if nErr != nil {
			return fmt.Errorf("entproto: format entmapper for %s: %w", node.Name, nErr)
		}
		outPath := filepath.Join(options.MapperDir, fileName)
		nErr = os.WriteFile(outPath, fixImport, 0644)
		if nErr != nil {
			return nErr
		}
	}
	return nil
}
