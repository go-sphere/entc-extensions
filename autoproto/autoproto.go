package autoproto

import (
	"errors"
	"fmt"
	"path"

	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
)

type ProtoOptions struct {
	Graph    *GraphOptions
	ProtoDir string
}

type Extension struct {
	entc.DefaultExtension
	options *ProtoOptions
}

func NewAutoProtoExtension(options *ProtoOptions) entc.Extension {
	return &Extension{options: options}
}

func (e *Extension) Hooks() []gen.Hook {
	return []gen.Hook{
		func(next gen.Generator) gen.Generator {
			return gen.GenerateFunc(func(g *gen.Graph) error {
				err := next.Generate(g)
				if err != nil {
					return err
				}
				g = FixGraph(g, e.options.Graph)
				return generateProto(g, e.options)
			})
		},
	}
}

func generateProto(g *gen.Graph, options *ProtoOptions) error {
	entProtoDir := path.Join(g.Target, "proto")
	if options.ProtoDir != "" {
		entProtoDir = options.ProtoDir
	}
	adapter, err := entproto.LoadAdapter(g)
	if err != nil {
		return fmt.Errorf("entproto: failed parsing entity graph: %w", err)
	}
	var errs []error
	for _, schema := range g.Schemas {
		name := schema.Name
		_, sErr := adapter.GetFileDescriptor(name)
		if sErr != nil && !errors.Is(sErr, entproto.ErrSchemaSkipped) {
			errs = append(errs, sErr)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("entproto: failed parsing some schemas: %w", errors.Join(errs...))
	}
	allDescriptors := make([]*desc.FileDescriptor, 0, len(adapter.AllFileDescriptors()))
	for _, fDesc := range adapter.AllFileDescriptors() {
		FixProto3Optional(g, fDesc)
		allDescriptors = append(allDescriptors, fDesc)
	}
	var printer protoprint.Printer
	printer.Compact = true
	if err = printer.PrintProtosToFileSystem(allDescriptors, entProtoDir); err != nil {
		return fmt.Errorf("entproto: failed writing .proto files: %w", err)
	}
	return nil
}
