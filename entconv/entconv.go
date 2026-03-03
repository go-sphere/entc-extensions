package entconv

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"

	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entconv/internal/generator"
	"github.com/go-sphere/entc-extensions/entconv/internal/pkgutil"
	"golang.org/x/tools/imports"
)

// Options holds all the configuration for code generation.
type Options struct {
	// ProtoGoFile is the path to the generated .pb.go file from protoc.
	// Example: "./api/entpb/entpb.pb.go"
	ProtoGoFile string

	// EntSchema is the directory containing ent schema definitions.
	// Example: "./internal/pkg/database/schema"
	EntSchema string

	// EntImportPath is the import path for the generated ent package.
	// Example: "github.com/example/project/internal/pkg/database/ent"
	EntImportPath string

	// Output is the file path where the converter code will be written.
	// Example: "./api/entpb/entpb_conv.go"
	Output string

	// IDType specifies the ID type for ent schema: int, int64, uint, uint64, string.
	// Default is "int64".
	IDType string

	// ProtoPackage is the Go package name for the proto types (from .pb.go).
	// Example: "entpb"
	ProtoPackage string

	// ProtoImportPath is the import path for the proto package.
	// Example: "github.com/example/project/api/entpb"
	ProtoImportPath string
}

// RequiredOptionError is returned when a required option is missing.
type RequiredOptionError struct {
	Field string
}

func (e *RequiredOptionError) Error() string {
	return fmt.Sprintf("required option %q is empty", e.Field)
}

// GenerateConverter generates ent <-> proto converter code and returns the formatted Go source.
func GenerateConverter(opts *Options) ([]byte, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	// Resolve ent package path
	entPkg, err := pkgutil.ResolveEntPackage(opts.EntSchema, opts.EntImportPath)
	if err != nil {
		return nil, fmt.Errorf("resolving ent package: %w", err)
	}

	// Parse and load ent graph
	idType := parseIDType(opts.IDType)
	g, err := loadEntGraph(opts.EntSchema, entPkg, idType)
	if err != nil {
		return nil, fmt.Errorf("loading ent graph: %w", err)
	}

	// Parse proto Go file to get message types
	protoTypes, err := parseProtoFile(opts.ProtoGoFile)
	if err != nil {
		return nil, fmt.Errorf("parsing proto file: %w", err)
	}

	// Auto-discover types from ent graph that match proto messages
	typesToGenerate := matchTypes(g, protoTypes)
	if len(typesToGenerate) == 0 {
		return nil, fmt.Errorf("no matching types found between ent schema and proto messages")
	}

	// Load entproto adapter
	adapter, err := loadAdapter(g)
	if err != nil {
		return nil, fmt.Errorf("loading adapter: %w", err)
	}

	// Generate code
	cg := generator.New(entPkg, opts.ProtoPackage, opts.ProtoImportPath, typesToGenerate, adapter, g)

	// Generate to buffer first
	var buf bytes.Buffer
	if err := cg.GenerateToWriter(&buf); err != nil {
		return nil, fmt.Errorf("generating code: %w", err)
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting source: %w", err)
	}

	// Run goimports to fix imports
	imported, err := imports.Process(opts.Output, formatted, nil)
	if err != nil {
		return nil, fmt.Errorf("running goimports: %w", err)
	}

	return imported, nil
}

// matchTypes matches ent types with proto messages and returns TypeInfo for matching pairs.
func matchTypes(g *gen.Graph, protoTypes map[string]*generator.ProtoMessage) []generator.TypeInfo {
	var typesToGenerate []generator.TypeInfo

	for _, node := range g.Nodes {
		protoType, ok := protoTypes[node.Name]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: proto message %q not found for ent type %q, skipping\n", node.Name, node.Name)
			continue
		}

		typesToGenerate = append(typesToGenerate, generator.TypeInfo{
			MessageName: node.Name,
			Message:     protoType,
			Type:        node,
		})
	}

	return typesToGenerate
}

// GenerateConverterFile generates converter code and writes it to the output file.
func GenerateConverterFile(opts *Options) error {
	source, err := GenerateConverter(opts)
	if err != nil {
		return err
	}

	return os.WriteFile(opts.Output, source, 0644)
}

// loadEntGraph loads the ent graph from the schema directory.
func loadEntGraph(schemaPath, entPackage string, idType *field.TypeInfo) (*gen.Graph, error) {
	return entc.LoadGraph(schemaPath, &gen.Config{
		Package: entPackage,
		IDType:  idType,
	})
}

// loadAdapter loads the entproto adapter for the given graph.
func loadAdapter(g *gen.Graph) (*entproto.Adapter, error) {
	return entproto.LoadAdapter(g)
}

// parseProtoFile parses a generated .pb.go file and returns a map of message name to field info.
func parseProtoFile(filePath string) (map[string]*generator.ProtoMessage, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	result := make(map[string]*generator.ProtoMessage)

	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		msg := &generator.ProtoMessage{
			Name:   typeSpec.Name.Name,
			Fields: parseStructFields(structType),
		}
		result[msg.Name] = msg
		return true
	})

	return result, nil
}

func parseStructFields(st *ast.StructType) []generator.ProtoField {
	var fields []generator.ProtoField
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		fieldName := field.Names[0].Name

		fields = append(fields, generator.ProtoField{
			Name: fieldName,
			Type: getTypeName(field.Type),
		})
	}
	return fields
}

// getTypeName extracts the type name from an AST expression.
func getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if name := getTypeName(t.X); name != "" {
			return "*" + name
		}
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.ArrayType:
		if name := getTypeName(t.Elt); name != "" {
			return "[]" + name
		}
	}
	return ""
}

// parseIDType parses the ID type string into a field.TypeInfo.
// Returns TypeInt64 as default if idType is empty or unrecognized.
func parseIDType(idType string) *field.TypeInfo {
	switch idType {
	case "int":
		return &field.TypeInfo{Type: field.TypeInt}
	case "", "int64": // default
		return &field.TypeInfo{Type: field.TypeInt64}
	case "uint":
		return &field.TypeInfo{Type: field.TypeUint}
	case "uint64":
		return &field.TypeInfo{Type: field.TypeUint64}
	case "string":
		return &field.TypeInfo{Type: field.TypeString}
	default:
		// Return default for unrecognized types
		return &field.TypeInfo{Type: field.TypeInt64}
	}
}

// validateOptions validates that all required options are provided.
func validateOptions(opts *Options) error {
	if opts.ProtoGoFile == "" {
		return &RequiredOptionError{Field: "ProtoGoFile"}
	}
	if opts.EntSchema == "" {
		return &RequiredOptionError{Field: "EntSchema"}
	}
	if opts.EntImportPath == "" {
		return &RequiredOptionError{Field: "EntImportPath"}
	}
	if opts.Output == "" {
		return &RequiredOptionError{Field: "Output"}
	}
	if opts.ProtoPackage == "" {
		return &RequiredOptionError{Field: "ProtoPackage"}
	}
	if opts.ProtoImportPath == "" {
		return &RequiredOptionError{Field: "ProtoImportPath"}
	}
	return nil
}
