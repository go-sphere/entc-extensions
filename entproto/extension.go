package entproto

import (
	"errors"
	"fmt"
	"path"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"github.com/jhump/protoreflect/desc"            //nolint:staticcheck
	"github.com/jhump/protoreflect/desc/protoprint" //nolint:staticcheck
	"go.uber.org/multierr"
)

// ExtensionOption is an option for the entproto extension.
type ExtensionOption func(*Extension)

// NewExtension returns a new Extension configured by opts.
func NewExtension(opts ...ExtensionOption) (*Extension, error) {
	e := &Extension{}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

// Extension is an entc.Extension that generates .proto files from an ent schema.
// To use within an entc.go file:
//
//	func main() {
//		if err := entc.Generate("./schema",
//			&gen.Config{},
//			entc.Extensions(
//				entproto.NewExtension(),
//			),
//		); err != nil {
//			log.Fatal("running ent codegen:", err)
//		}
//	}
type Extension struct {
	entc.DefaultExtension
	protoDir string
	autoFill bool
}

// WithProtoDir sets the directory where the generated .proto files will be written.
func WithProtoDir(dir string) ExtensionOption {
	return func(e *Extension) {
		e.protoDir = dir
	}
}

// WithAutoFill enables automatic generation of entproto annotations.
// When enabled, schemas without Message annotation will get one automatically,
// and all fields/edges without Field annotation will be annotated with auto-generated field numbers.
func WithAutoFill() ExtensionOption {
	return func(e *Extension) {
		e.autoFill = true
	}
}

// Hooks implements entc.Extension.
func (e *Extension) Hooks() []gen.Hook {
	return []gen.Hook{e.hook()}
}

func (e *Extension) hook() gen.Hook {
	return func(next gen.Generator) gen.Generator {
		return gen.GenerateFunc(func(g *gen.Graph) error {
			// Because Generate has side effects (it is writing to the filesystem under gen.Config.Target),
			// we first run all generators, and only then invoke our code. This isn't great, and there's an
			// [open issue](https://github.com/ent/ent/issues/1311) to support this use-case better.
			err := next.Generate(g)
			if err != nil {
				return err
			}
			if e.autoFill {
				if err := FixGraph(g); err != nil {
					return err
				}
			}
			return e.generate(g)
		})
	}
}

// Hook returns a gen.Hook that invokes Generate.
// To use it programatically:
//
//	entc.Generate("./ent/schema", &gen.Config{
//	  Hooks: []gen.Hook{
//	    entproto.Hook(),
//	  },
//	})
//
// Deprecated: use Extension instead.
func Hook() gen.Hook {
	x := &Extension{}
	return x.hook()
}

// Generate takes a *gen.Graph and creates .proto files.
func Generate(g *gen.Graph) error {
	x := &Extension{}
	return x.generate(g)
}

func (e *Extension) generate(g *gen.Graph) error {
	entProtoDir := path.Join(g.Target, "proto")
	if e.protoDir != "" {
		entProtoDir = e.protoDir
	}
	adapter, err := LoadAdapter(g)
	if err != nil {
		return fmt.Errorf("entproto: failed parsing ent graph: %w", err)
	}
	var errs error
	for _, schema := range g.Schemas {
		name := schema.Name
		_, err := adapter.GetFileDescriptor(name)
		if err != nil && !errors.Is(err, ErrSchemaSkipped) {
			errs = multierr.Append(errs, err)
		}
	}
	if errs != nil {
		return fmt.Errorf("entproto: failed parsing some schemas: %w", errs)
	}
	allDescriptors := make([]*desc.FileDescriptor, 0, len(adapter.AllFileDescriptors()))
	for _, filedesc := range adapter.AllFileDescriptors() {
		allDescriptors = append(allDescriptors, filedesc)
	}
	// Print the .proto files.
	var printer protoprint.Printer
	if err = printer.PrintProtosToFileSystem(allDescriptors, entProtoDir); err != nil {
		return fmt.Errorf("entproto: failed writing .proto files: %w", err)
	}

	return nil
}
