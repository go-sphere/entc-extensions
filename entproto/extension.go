package entproto

import (
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"github.com/jhump/protoreflect/desc"            //nolint:staticcheck
	"github.com/jhump/protoreflect/desc/protoprint" //nolint:staticcheck
	"go.uber.org/multierr"
)

// ExtensionOption configures an [Extension] created by [NewExtension].
type ExtensionOption func(*Extension)

// NewExtension returns an [Extension] configured by opts, applied in order;
// when an option is repeated, the last one wins. With no options, .proto files
// are written to <gen.Config.Target>/proto and only schemas annotated with
// [Message] are generated.
//
// The returned error is currently always nil; callers should still check it.
// Pass the extension to [entgo.io/ent/entc.Extensions].
func NewExtension(opts ...ExtensionOption) (*Extension, error) {
	e := &Extension{}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

// Extension is an [entgo.io/ent/entc.Extension] that generates .proto files
// from an ent schema. Create it with [NewExtension]; the zero value behaves
// like NewExtension() with no options.
//
// Its hook first runs the wrapped ent generator, so ent's own code is written
// before any .proto file. It then, in order: applies [FixGraph] when
// [WithAutoFill] is set, builds descriptors with [LoadAdapter], and writes one
// .proto file per protobuf package, overwriting existing files. Schemas without
// a [Message] annotation (or with [SkipGen]) are omitted and, unless AutoFill
// is enabled, listed in a warning on standard error. Any schema that fails to
// convert aborts generation with an error prefixed "entproto:"; files written
// by ent itself are not rolled back.
//
// Use within an entc.go file:
//
//	func main() {
//		ext, err := entproto.NewExtension()
//		if err != nil {
//			log.Fatal(err)
//		}
//		if err := entc.Generate("./schema",
//			&gen.Config{},
//			entc.Extensions(ext),
//		); err != nil {
//			log.Fatal("running ent codegen:", err)
//		}
//	}
type Extension struct {
	entc.DefaultExtension
	protoDir string
	autoFill bool
}

// WithProtoDir sets the root directory for generated .proto files. A relative
// dir is resolved against the process working directory. An empty dir keeps
// the default, path.Join(gen.Config.Target, "proto"). Package a.b.c is written
// to <dir>/a/b/c/c.proto.
func WithProtoDir(dir string) ExtensionOption {
	return func(e *Extension) {
		e.protoDir = dir
	}
}

// WithAutoFill makes the extension call [FixGraph] before generating, so
// schemas and members without entproto annotations are annotated
// automatically: every schema without a [Message] annotation gets Message()
// (proto package "entpb"), the ID gets field number 1, and each other field or
// edge without [Field] or [Skip] gets a number derived from a stable hash of
// its schema, kind, and name. Existing annotations are kept; schemas marked
// with [SkipGen] are left untouched. A hash collision with an existing number
// fails generation and must be resolved by annotating the colliding members
// explicitly. AutoFill also suppresses the warning about unannotated schemas.
func WithAutoFill() ExtensionOption {
	return func(e *Extension) {
		e.autoFill = true
	}
}

// Hooks implements [entgo.io/ent/entc.Extension]. It returns a single hook
// that performs the generation described on [Extension].
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

// Hook returns a [entgo.io/ent/entc/gen.Hook] equivalent to the hook of an
// [Extension] created without options. To use it programmatically:
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

// Generate writes .proto files for g as an [Extension] without options would:
// files go to path.Join(g.Target, "proto"), and only schemas annotated with
// [Message] are generated. It does not run [FixGraph] and does not run ent's
// own code generation. Errors are returned with an "entproto:" prefix.
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
	var skipped []string
	for _, schema := range g.Schemas {
		name := schema.Name
		_, err := adapter.GetFileDescriptor(name)
		if err != nil && !errors.Is(err, ErrSchemaSkipped) {
			errs = multierr.Append(errs, err)
			continue
		}
		if err != nil && errors.Is(err, ErrSchemaSkipped) && !e.autoFill {
			// With AutoFill disabled a schema without an entproto.Message
			// annotation is silently absent from every .proto file. Warn so a
			// forgotten annotation does not go unnoticed.
			skipped = append(skipped, name)
		}
	}
	if errs != nil {
		return fmt.Errorf("entproto: failed parsing some schemas: %w", errs)
	}
	if len(skipped) > 0 {
		fmt.Fprintln(os.Stderr, skippedSchemasWarning(skipped))
	}
	generated := adapter.GeneratedFileDescriptors()
	allDescriptors := make([]*desc.FileDescriptor, 0, len(generated))
	for _, filedesc := range generated {
		allDescriptors = append(allDescriptors, filedesc)
	}
	// Print the .proto files.
	var printer protoprint.Printer
	if err = printer.PrintProtosToFileSystem(allDescriptors, entProtoDir); err != nil {
		return fmt.Errorf("entproto: failed writing .proto files: %w", err)
	}

	return nil
}

// skippedSchemasWarning renders the stderr warning listing schemas that will not
// appear in any generated .proto file because they lack an entproto.Message
// annotation (AutoFill disabled).
func skippedSchemasWarning(skipped []string) string {
	slices.Sort(skipped)
	return "entproto: warning: schema(s) " + strings.Join(skipped, ", ") +
		" have no entproto.Message(Generate=true) annotation and will not appear in any generated .proto file"
}
