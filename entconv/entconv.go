package entconv

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entconv/internal/generator"
	"github.com/go-sphere/entc-extensions/entconv/internal/pkgutil"
	"github.com/go-sphere/entc-extensions/entproto"
	"golang.org/x/tools/imports"
)

type Options struct {
	SchemaPath       string
	EntPackagePath   string
	IDType           string
	ProtoFile        string
	ConvPackage      string
	ProtoPackagePath string
	ProtoAlias       string
	OutDir           string
}

type RequiredOptionError struct {
	Field string
}

func (e *RequiredOptionError) Error() string {
	return fmt.Sprintf("required option %q is empty", e.Field)
}

func GenerateConverter(opts *Options) ([]byte, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	entPkg, err := pkgutil.ResolveEntPackage(opts.SchemaPath, opts.EntPackagePath)
	if err != nil {
		return nil, fmt.Errorf("resolving ent package: %w", err)
	}

	idType := parseIDType(opts.IDType)
	g, err := loadEntGraph(opts.SchemaPath, entPkg, idType)
	if err != nil {
		return nil, fmt.Errorf("loading ent graph: %w", err)
	}

	protoTypes, err := parseProtoFile(opts.ProtoFile)
	if err != nil {
		return nil, fmt.Errorf("parsing proto file: %w", err)
	}

	typesToGenerate := matchTypes(g, protoTypes)
	if len(typesToGenerate) == 0 {
		return nil, fmt.Errorf("no matching types found between ent schema and proto messages")
	}

	adapter, err := loadAdapter(g)
	if err != nil {
		return nil, fmt.Errorf("loading adapter: %w", err)
	}

	var protoAlias string
	if opts.ProtoPackagePath != "" {
		protoAlias = opts.ProtoAlias
		if protoAlias == "" {
			protoAlias = opts.ConvPackage
		}
	}
	cg := generator.New(entPkg, opts.ConvPackage, opts.ProtoPackagePath, protoAlias, typesToGenerate, adapter, g)

	var buf bytes.Buffer
	if err := cg.GenerateToWriter(&buf); err != nil {
		return nil, fmt.Errorf("generating code: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting source: %w", err)
	}

	imported, err := imports.Process("", formatted, nil)
	if err != nil {
		return nil, fmt.Errorf("running goimports: %w", err)
	}

	return imported, nil
}

func GenerateConverterFile(opts *Options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}

	entPkg, err := pkgutil.ResolveEntPackage(opts.SchemaPath, opts.EntPackagePath)
	if err != nil {
		return fmt.Errorf("resolving ent package: %w", err)
	}

	idType := parseIDType(opts.IDType)
	g, err := loadEntGraph(opts.SchemaPath, entPkg, idType)
	if err != nil {
		return fmt.Errorf("loading ent graph: %w", err)
	}

	protoTypes, err := parseProtoFile(opts.ProtoFile)
	if err != nil {
		return fmt.Errorf("parsing proto file: %w", err)
	}

	typesToGenerate := matchTypes(g, protoTypes)
	if len(typesToGenerate) == 0 {
		return fmt.Errorf("no matching types found between ent schema and proto messages")
	}

	adapter, err := loadAdapter(g)
	if err != nil {
		return fmt.Errorf("loading adapter: %w", err)
	}

	var protoAlias string
	if opts.ProtoPackagePath != "" {
		protoAlias = opts.ProtoAlias
		if protoAlias == "" {
			protoAlias = opts.ConvPackage
		}
	}

	cg := generator.New(entPkg, opts.ConvPackage, opts.ProtoPackagePath, protoAlias, typesToGenerate, adapter, g)
	return cg.GenerateAll(opts.OutDir)
}

func loadEntGraph(schemaPath, entPackage string, idType *field.TypeInfo) (*gen.Graph, error) {
	return entc.LoadGraph(schemaPath, &gen.Config{
		Package: entPackage,
		IDType:  idType,
	})
}

func loadAdapter(g *gen.Graph) (*entproto.Adapter, error) {
	return entproto.LoadAdapter(g)
}

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

func parseIDType(idType string) *field.TypeInfo {
	switch idType {
	case "int":
		return &field.TypeInfo{Type: field.TypeInt}
	case "", "int64":
		return &field.TypeInfo{Type: field.TypeInt64}
	case "uint":
		return &field.TypeInfo{Type: field.TypeUint}
	case "uint64":
		return &field.TypeInfo{Type: field.TypeUint64}
	case "string":
		return &field.TypeInfo{Type: field.TypeString}
	default:
		return &field.TypeInfo{Type: field.TypeInt64}
	}
}

func validateOptions(opts *Options) error {
	if opts.ProtoFile == "" {
		return &RequiredOptionError{Field: "ProtoFile"}
	}
	if opts.SchemaPath == "" {
		return &RequiredOptionError{Field: "SchemaPath"}
	}
	if opts.EntPackagePath == "" {
		return &RequiredOptionError{Field: "EntPackage"}
	}
	if opts.ConvPackage == "" {
		return &RequiredOptionError{Field: "ProtoPackage"}
	}
	if opts.ProtoPackagePath == "" {
		return &RequiredOptionError{Field: "ProtoPackagePath"}
	}
	if opts.OutDir == "" {
		return &RequiredOptionError{Field: "OutDir"}
	}
	return nil
}
