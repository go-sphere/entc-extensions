package mapper

import (
	_ "embed"
	"fmt"
	"go/format"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc/gen"
	"github.com/jhump/protoreflect/desc"
	"google.golang.org/protobuf/types/descriptorpb"
)

//go:embed mapper.tmpl
var mapperTemplates string

type Generator struct {
	PackageName      string
	ProtoPackageName string
	ProtoImportPath  GoImportPath
	EntImportPath    GoImportPath
	File             *desc.FileDescriptor
	Message          *desc.MessageDescriptor
	EntType          *gen.Type
	FieldMap         entproto.FieldMap
	Imports          map[string]string // path -> alias
	ImportAliases    map[string]string // alias -> path
	converterCache   map[*entproto.FieldMappingDescriptor]*Converter
}

type ImportSpec struct {
	Path  string
	Alias string
}

func NewGenerator(file *desc.FileDescriptor, graph *gen.Graph, adapter *entproto.Adapter, entType *gen.Type, message *desc.MessageDescriptor) (*Generator, error) {
	fieldMap, err := adapter.FieldMap(entType.Name)
	if err != nil {
		return nil, err
	}
	importPath, pkgName := parseProtoPackage(file)
	return &Generator{
		ProtoPackageName: pkgName,
		ProtoImportPath:  importPath,
		EntImportPath:    GoImportPath(graph.Package),
		File:             file,
		Message:          message,
		EntType:          entType,
		FieldMap:         fieldMap,
		Imports:          make(map[string]string),
		ImportAliases:    make(map[string]string),
		converterCache:   make(map[*entproto.FieldMappingDescriptor]*Converter),
	}, nil
}

func parseProtoPackage(file *desc.FileDescriptor) (GoImportPath, string) {
	pkgName := strings.ReplaceAll(file.GetPackage(), ".", "_")
	var importPath GoImportPath
	opts, ok := file.GetOptions().(*descriptorpb.FileOptions)
	if !ok || opts == nil {
		return importPath, pkgName
	}

	pathOpt, pkgOpt := parseGoPackageOption(opts.GetGoPackage())
	if pathOpt != "" {
		importPath = pathOpt
	}
	switch {
	case pkgOpt != "":
		pkgName = pkgOpt
	case importPath != "":
		pkgName = path.Base(string(importPath))
	}
	return importPath, pkgName
}

func parseGoPackageOption(goPackage string) (GoImportPath, string) {
	if goPackage == "" {
		return "", ""
	}
	parts := strings.Split(goPackage, ";")
	importPath := strings.TrimSpace(parts[0])
	pkgName := ""
	if len(parts) > 1 {
		pkgName = strings.TrimSpace(parts[1])
	}
	return GoImportPath(importPath), pkgName
}

func (g *Generator) EntIdent(subpath string, ident string) GoIdent {
	ip := path.Join(string(g.EntImportPath), subpath)
	return GoIdent{
		GoImportPath: GoImportPath(ip),
		GoName:       ident,
	}
}

func (g *Generator) QualifiedGoIdent(ident GoIdent) string {
	if string(ident.GoImportPath) == "" {
		return ident.GoName
	}
	alias := g.addImport(string(ident.GoImportPath))
	return alias + "." + ident.GoName
}

func (g *Generator) addImport(path string) string {
	if alias, ok := g.Imports[path]; ok {
		return alias
	}
	// Create a unique alias
	base := filepath.Base(path)
	base = strings.ReplaceAll(base, ".", "_")
	base = strings.ReplaceAll(base, "-", "_")
	alias := base
	count := 0
	for {
		if _, exists := g.ImportAliases[alias]; !exists {
			break
		}
		count++
		alias = fmt.Sprintf("%s%d", base, count)
	}
	g.Imports[path] = alias
	g.ImportAliases[alias] = path
	return alias
}

type GoImportPath string

func (p GoImportPath) Ident(name string) GoIdent {
	return GoIdent{
		GoImportPath: p,
		GoName:       name,
	}
}

type GoIdent struct {
	GoImportPath GoImportPath
	GoName       string
}

func (g *Generator) Generate() ([]byte, error) {
	if err := g.preloadConverters(); err != nil {
		return nil, err
	}

	tmpl, err := gen.NewTemplate("entmapper").
		Funcs(template.FuncMap{
			"ident":    g.QualifiedGoIdent,
			"entIdent": g.EntIdent,
			"pbIdent":  g.PBIdent,
			"converter": func(fld *entproto.FieldMappingDescriptor) (*Converter, error) {
				return g.converterFor(fld)
			},
			"camel":    gen.Funcs["camel"],
			"snake":    gen.Funcs["snake"],
			"singular": gen.Funcs["singular"],
			"upper":    strings.ToUpper,
			"qualify": func(pkg, ident string) string {
				return g.QualifiedGoIdent(GoImportPath(pkg).Ident(ident))
			},
			"protoIdentNormalize": entproto.NormalizeEnumIdentifier,
			"method": func(m *desc.MethodDescriptor) *MethodInput {
				return &MethodInput{
					G:      g,
					Method: m,
				}
			},
		}).Parse(mapperTemplates)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	err = tmpl.ExecuteTemplate(&buf, "entmapper", g)
	if err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	source, err := format.Source([]byte(buf.String()))
	if err != nil {
		return nil, err
	}

	return source, nil
}

func (g *Generator) preloadConverters() error {
	for _, fld := range g.FieldMap.Fields() {
		if _, err := g.converterFor(fld); err != nil {
			return err
		}
	}
	for _, edge := range g.FieldMap.Edges() {
		if _, err := g.converterFor(edge); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) converterFor(fld *entproto.FieldMappingDescriptor) (*Converter, error) {
	if conv, ok := g.converterCache[fld]; ok {
		return conv, nil
	}
	conv, err := g.NewConverter(fld)
	if err != nil {
		return nil, fmt.Errorf("entproto: build converter for %s: %w", fld.PbStructField(), err)
	}
	g.converterCache[fld] = conv
	return conv, nil
}

func (g *Generator) ImportSpecs() []ImportSpec {
	specs := make([]ImportSpec, 0, len(g.Imports))
	for p, alias := range g.Imports {
		specs = append(specs, ImportSpec{
			Path:  p,
			Alias: alias,
		})
	}
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Alias == specs[j].Alias {
			return specs[i].Path < specs[j].Path
		}
		return specs[i].Alias < specs[j].Alias
	})
	return specs
}

type MethodInput struct {
	G      *Generator
	Method *desc.MethodDescriptor
}

func (g *Generator) GoPackageName() string {
	if g.PackageName != "" {
		return g.PackageName
	}
	if g.ProtoPackageName != "" {
		return g.ProtoPackageName
	}
	return strings.ReplaceAll(g.File.GetPackage(), ".", "_")
}

func (g *Generator) GoImportPath() GoImportPath {
	if g.ProtoImportPath == "" {
		return ""
	}
	if g.samePackageAsProto() {
		return ""
	}
	return g.ProtoImportPath
}

func (g *Generator) PBIdent(name string) string {
	if name == "" {
		return ""
	}
	if g.ProtoImportPath == "" || g.samePackageAsProto() {
		return name
	}
	return g.QualifiedGoIdent(g.ProtoImportPath.Ident(name))
}

func (g *Generator) samePackageAsProto() bool {
	if g.ProtoPackageName == "" {
		return true
	}
	return g.GoPackageName() == g.ProtoPackageName
}
