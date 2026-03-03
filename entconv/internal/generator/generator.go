package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"entgo.io/ent/entc/gen"
	"github.com/go-sphere/entc-extensions/entconv/internal/converter"
	"github.com/go-sphere/entc-extensions/entproto"
	"golang.org/x/tools/imports"
)

// templateCache holds the parsed templates to avoid re-parsing on each generation.
var (
	templateCache     *gen.Template
	templateCacheOnce sync.Once
	templateCacheErr  error
)

// ProtoMessage represents a parsed proto message from .pb.go file.
type ProtoMessage struct {
	Name   string
	Fields []ProtoField
}

// ProtoField represents a field in a proto message.
type ProtoField struct {
	Name string
	Type string
}

// TypeInfo holds information about a type being processed.
type TypeInfo struct {
	MessageName string
	Message     *ProtoMessage
	Type        *gen.Type
}

// Generator handles code generation for Ent <-> Proto converters.
type Generator struct {
	EntPackage  string
	ConvPackage string
	// ProtoPackagePath is the import path for the proto package.
	ProtoPackagePath string
	// ProtoAlias is the alias used for the proto package in generated code.
	ProtoAlias string
	Types      []TypeInfo
	Adapter    *entproto.Adapter
	Graph      *gen.Graph
	// currentType is the type being generated in single-type mode
	currentType *TypeInfo
	// nodeIndex provides O(1) lookup for node names
	nodeIndex map[string]*gen.Type
	// typeIndex provides O(1) lookup for type info by name
	typeIndex map[string]*TypeInfo
}

// New creates a new Generator.
func New(entPackage, pkg, pkgPath, protoAlias string, types []TypeInfo, adapter *entproto.Adapter, graph *gen.Graph) *Generator {
	// Build index for O(1) node lookup
	nodeIndex := make(map[string]*gen.Type, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeIndex[node.Name] = node
	}

	// Build index for O(1) type info lookup
	typeIndex := make(map[string]*TypeInfo, len(types))
	for i := range types {
		typeIndex[types[i].Type.Name] = &types[i]
	}

	return &Generator{
		EntPackage:       entPackage,
		ConvPackage:      pkg,
		ProtoPackagePath: pkgPath,
		ProtoAlias:       protoAlias,
		Types:            types,
		Adapter:          adapter,
		Graph:            graph,
		nodeIndex:        nodeIndex,
		typeIndex:        typeIndex,
	}
}

//go:embed template/*
var templates embed.FS

// getTemplate returns the parsed template, using a cached version if available.
func (g *Generator) getTemplate() (*gen.Template, error) {
	templateCacheOnce.Do(func() {
		tmplContent, err := fs.ReadFile(templates, "template/converter.tmpl")
		if err != nil {
			templateCacheErr = fmt.Errorf("failed to read template: %w", err)
			return
		}

		templateCache, templateCacheErr = gen.NewTemplate("converter").
			Funcs(g.templateFuncs()).
			Parse(string(tmplContent))
	})

	return templateCache, templateCacheErr
}

// Generate produces the converter code and writes to the specified output path.
func (g *Generator) Generate(outputPath string) error {
	var buf bytes.Buffer
	if err := g.generateBody(&buf); err != nil {
		return err
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// GenerateAll generates separate converter files for each type in the output directory.
func (g *Generator) GenerateAll(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	for _, typeInfo := range g.Types {
		g.currentType = &typeInfo
		var buf bytes.Buffer
		if err := g.generateSingleType(&buf, typeInfo); err != nil {
			return fmt.Errorf("generating %s: %w", typeInfo.Type.Name, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("formatting %s: %w", typeInfo.Type.Name, err)
		}

		outputPath := fmt.Sprintf("%s/%s.go", outputDir, strings.ToLower(typeInfo.Type.Name))
		// Use goimports to clean up unused imports
		optimized, err := imports.Process(outputPath, formatted, nil)
		if err != nil {
			return fmt.Errorf("optimizing imports for %s: %w", typeInfo.Type.Name, err)
		}

		if err := os.WriteFile(outputPath, optimized, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", typeInfo.Type.Name, err)
		}
	}

	return nil
}

// GenerateToWriter produces the converter code and writes to the provided io.Writer.
// This method outputs raw code suitable for further processing (formatting, goimports, etc.) by the caller.
func (g *Generator) GenerateToWriter(w io.Writer) error {
	return g.generateBody(w)
}

// generateSingleType generates converter code for a single type.
func (g *Generator) generateSingleType(w io.Writer, typeInfo TypeInfo) error {
	// Create a temporary generator with only one type
	tempGen := &Generator{
		EntPackage:       g.EntPackage,
		ConvPackage:      g.ConvPackage,
		ProtoPackagePath: g.ProtoPackagePath,
		ProtoAlias:       g.ProtoAlias,
		Types:            []TypeInfo{typeInfo},
		Adapter:          g.Adapter,
		Graph:            g.Graph,
		currentType:      &typeInfo,
		nodeIndex:        g.nodeIndex,
		typeIndex:        g.typeIndex,
	}

	tmpl, err := tempGen.getTemplate()
	if err != nil {
		return err
	}

	// Write header with imports
	if err := tempGen.writeImports(w); err != nil {
		return err
	}

	// Execute template
	if err := tmpl.Execute(w, tempGen); err != nil {
		return fmt.Errorf("template execution failed: %w", err)
	}

	return nil
}

// generateBody generates the converter code body (with imports) to the writer.
func (g *Generator) generateBody(w io.Writer) error {
	tmpl, err := g.getTemplate()
	if err != nil {
		return err
	}

	// Write header with imports
	if err := g.writeImports(w); err != nil {
		return err
	}

	// Execute template
	if err := tmpl.Execute(w, g); err != nil {
		return fmt.Errorf("template execution failed: %w", err)
	}

	return nil
}

// Imports returns the list of imports needed by the generated code.
func (g *Generator) Imports() []string {
	imp := []string{
		`runtime "github.com/go-sphere/entc-extensions/entproto/runtime"`,
		`timestamppb "google.golang.org/protobuf/types/known/timestamppb"`,
		`regexp "regexp"`,
		`strings "strings"`,
	}

	// Add ent import
	imp = append(imp, fmt.Sprintf(`ent "%s"`, g.EntPackage))

	// Add proto package import if ProtoPackagePath is specified (for separate package generation)
	if g.ProtoPackagePath != "" && g.ProtoAlias != "" {
		imp = append(imp, fmt.Sprintf(`%s "%s"`, g.ProtoAlias, g.ProtoPackagePath))
	}

	// Check if any type needs the post package (for enums)
	for _, t := range g.Types {
		fieldMap, err := g.Adapter.FieldMap(t.Type.Name)
		if err != nil {
			continue
		}
		for range fieldMap.Enums() {
			enumPkg, _ := g.entEnumPkg(t.Type.Name)
			if enumPkg != g.EntPackage {
				imp = append(imp, fmt.Sprintf(`post "%s"`, enumPkg))
			}
		}
	}

	return imp
}

// writeImports writes the import block to the output.
func (g *Generator) writeImports(w io.Writer) error {
	imports := g.Imports()

	if _, err := w.Write([]byte("// Code generated by protoc-gen-entconv. DO NOT EDIT.\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("package " + g.ConvPackage + "\n\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("import (\n")); err != nil {
		return err
	}
	for _, imp := range imports {
		if _, err := w.Write([]byte("\t" + imp + "\n")); err != nil {
			return err
		}
	}
	if _, err := w.Write([]byte(")\n\n")); err != nil {
		return err
	}
	return nil
}

func (g *Generator) templateFuncs() template.FuncMap {
	return template.FuncMap{
		"ident":               g.ident,
		"entIdent":            g.entIdent,
		"entEnumPkg":          g.entEnumPkg,
		"newConverter":        g.newConverter,
		"unquote":             strconv.Unquote,
		"camel":               gen.Funcs["camel"],
		"snake":               gen.Funcs["snake"],
		"pascal":              g.pascal,
		"upper":               strings.ToUpper,
		"singular":            gen.Funcs["singular"],
		"qualify":             g.qualify,
		"protoIdentNormalize": entproto.NormalizeEnumIdentifier,
		"statusErr":           g.statusErr,
		"statusErrf":          g.statusErrf,
		"getFieldMap":         g.getFieldMap,
		"protoIdent":          g.protoIdent,
		"entPackageIdent":     g.entPackageIdent,
	}
}

// pascal converts a string to PascalCase.
func (g *Generator) pascal(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ident returns a qualified identifier (accepts full qualified path like "pkg.Name").
func (g *Generator) ident(qualifiedIdent string) string {
	return qualifiedIdent
}

// qualify returns a qualified identifier for the given package and identifier.
func (g *Generator) qualify(pkg, ident string) string {
	return pkg + "." + ident
}

// statusErr creates a gRPC status error.
func (g *Generator) statusErr(code, msg string) string {
	return fmt.Sprintf("status.Error(codes.%s, %q)", code, msg)
}

// statusErrf creates a formatted gRPC status error.
func (g *Generator) statusErrf(code, format string, args ...string) string {
	return fmt.Sprintf("status.Errorf(codes.%s, %s, %s)", code, strconv.Quote(format), strings.Join(args, ","))
}

// getFieldMap returns the FieldMap for a given type name.
func (g *Generator) getFieldMap(typeName string) (entproto.FieldMap, error) {
	return g.Adapter.FieldMap(typeName)
}

// entIdent returns a qualified identifier for the ent package (using import alias).
func (g *Generator) entIdent(subpath string, ident string) string {
	if subpath == "" {
		return "ent." + ident
	}
	// For subpackages like "post", use the package name as alias
	pkgName := path.Base(subpath)
	return pkgName + "." + ident
}

// protoIdent returns a qualified identifier for the proto package (using import alias).
func (g *Generator) protoIdent(ident string) string {
	// If ProtoAlias is specified, use it for separate package generation
	if g.ProtoAlias != "" {
		return g.ProtoAlias + "." + ident
	}
	// Otherwise, use bare identifier (same package)
	return ident
}

// entPackageIdent returns a qualified identifier for the ent type (using import alias).
func (g *Generator) entPackageIdent(typeName string) string {
	return "ent." + typeName
}

// entEnumPkg returns the full import path for the enum's package based on schema type.
// Uses O(1) map lookup instead of linear search.
func (g *Generator) entEnumPkg(typeName string) (string, error) {
	// O(1) lookup using the index
	if _, ok := g.nodeIndex[typeName]; ok {
		return path.Join(g.EntPackage, strings.ToLower(typeName)), nil
	}

	// Default to main package if not found in graph
	return g.EntPackage, nil
}

// newConverter creates a Converter for the given field mapping and type name.
func (g *Generator) newConverter(fld *entproto.FieldMappingDescriptor, typeName string) (*converter.Converter, error) {
	typeInfo, ok := g.typeIndex[typeName]
	if !ok {
		return nil, fmt.Errorf("type %q not found", typeName)
	}
	return converter.NewConverter(fld, typeName, typeInfo.Type)
}

// FieldMap returns the FieldMap for a given type name.
func (g *Generator) FieldMap(typeName string) (entproto.FieldMap, error) {
	return g.Adapter.FieldMap(typeName)
}
