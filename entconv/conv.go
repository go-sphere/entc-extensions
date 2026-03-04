package entconv

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"runtime/debug"
	"slices"
	"strings"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entconv/internal/generator"
	"github.com/go-sphere/entc-extensions/entconv/internal/pkgutil"
	"github.com/go-sphere/entc-extensions/entproto"
	"golang.org/x/tools/imports"
)

type Options struct {
	SchemaPath         string
	EntPackagePath     string
	IDType             string
	ProtoFile          string
	ConvPackage        string
	ProtoPackagePath   string
	ProtoAlias         string
	OutDir             string
	MissingProtoPolicy MissingProtoPolicy
	WarningHandler     func(error)
}

type MissingProtoPolicy string

const (
	MissingProtoPolicyStrict MissingProtoPolicy = "strict"
	MissingProtoPolicyWarn   MissingProtoPolicy = "warn"
)

type MissingProtoMessagesError struct {
	Missing []string
}

func (e *MissingProtoMessagesError) Error() string {
	return fmt.Sprintf("missing proto messages for ent types: %s", strings.Join(e.Missing, ", "))
}

func (e *MissingProtoMessagesError) Is(target error) bool {
	var t *MissingProtoMessagesError
	return errors.As(target, &t)
}

type Option func(*Options)

func DefaultOptions() *Options {
	modulePath := currentModulePath()
	return &Options{
		IDType:             "int64",
		SchemaPath:         "./internal/pkg/database/schema",
		EntPackagePath:     path.Join(modulePath, "/internal/pkg/database/ent"),
		ProtoFile:          "./api/entpb/entpb.pb.go",
		ConvPackage:        "entmap",
		ProtoPackagePath:   path.Join(modulePath, "/api/entpb"),
		ProtoAlias:         "entpb",
		OutDir:             "./internal/pkg/render/entmap",
		MissingProtoPolicy: MissingProtoPolicyStrict,
	}
}

func NewOptions(opts ...Option) *Options {
	o := DefaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

func WithSchemaPath(v string) Option {
	return func(o *Options) {
		o.SchemaPath = v
	}
}

func WithEntPackagePath(v string) Option {
	return func(o *Options) {
		o.EntPackagePath = v
	}
}

func WithIDType(v string) Option {
	return func(o *Options) {
		o.IDType = v
	}
}

func WithProtoFile(v string) Option {
	return func(o *Options) {
		o.ProtoFile = v
	}
}

func WithConvPackage(v string) Option {
	return func(o *Options) {
		o.ConvPackage = v
	}
}

func WithProtoPackagePath(v string) Option {
	return func(o *Options) {
		o.ProtoPackagePath = v
	}
}

func WithProtoAlias(v string) Option {
	return func(o *Options) {
		o.ProtoAlias = v
	}
}

func WithOutDir(v string) Option {
	return func(o *Options) {
		o.OutDir = v
	}
}

func WithMissingProtoPolicy(v MissingProtoPolicy) Option {
	return func(o *Options) {
		o.MissingProtoPolicy = v
	}
}

func WithWarningHandler(h func(error)) Option {
	return func(o *Options) {
		o.WarningHandler = h
	}
}

type RequiredOptionError struct {
	Field string
}

func (e *RequiredOptionError) Error() string {
	return fmt.Sprintf("required option %q is empty", e.Field)
}

func GenerateConverter(opts *Options) ([]byte, error) {
	cg, err := prepareGenerator(opts)
	if err != nil {
		return nil, err
	}

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

func GenerateConverterWithOptions(opts ...Option) ([]byte, error) {
	return GenerateConverter(NewOptions(opts...))
}

func GenerateConverterFile(opts *Options) error {
	cg, err := prepareGenerator(opts)
	if err != nil {
		return err
	}
	return cg.GenerateAll(opts.OutDir)
}

func GenerateConverterFileWithOptions(opts ...Option) error {
	return GenerateConverterFile(NewOptions(opts...))
}

func prepareGenerator(opts *Options) (*generator.Generator, error) {
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

	typesToGenerate, missing := matchTypes(g, protoTypes)
	if missing != nil {
		switch normalizePolicy(opts.MissingProtoPolicy) {
		case MissingProtoPolicyWarn:
			if opts.WarningHandler != nil {
				opts.WarningHandler(missing)
			}
		default:
			return nil, missing
		}
	}

	if len(typesToGenerate) == 0 {
		return nil, fmt.Errorf("no matching types found between ent schema and proto messages")
	}

	adapter, err := loadAdapter(g)
	if err != nil {
		return nil, fmt.Errorf("loading adapter: %w", err)
	}

	return generator.New(
		entPkg,
		opts.ConvPackage,
		opts.ProtoPackagePath,
		resolveProtoAlias(opts),
		typesToGenerate,
		adapter,
		g,
	), nil
}

func currentModulePath() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Path
}

func resolveProtoAlias(opts *Options) string {
	if opts.ProtoPackagePath == "" {
		return ""
	}
	if opts.ProtoAlias != "" {
		return opts.ProtoAlias
	}
	return opts.ConvPackage
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

func matchTypes(g *gen.Graph, protoTypes map[string]*generator.ProtoMessage) ([]generator.TypeInfo, *MissingProtoMessagesError) {
	var typesToGenerate []generator.TypeInfo
	missing := make([]string, 0)

	for _, node := range g.Nodes {
		protoType, ok := protoTypes[node.Name]
		if !ok {
			missing = append(missing, node.Name)
			continue
		}

		typesToGenerate = append(typesToGenerate, generator.TypeInfo{
			MessageName: node.Name,
			Message:     protoType,
			Type:        node,
		})
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return typesToGenerate, &MissingProtoMessagesError{Missing: missing}
	}
	return typesToGenerate, nil
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
	if p := normalizePolicy(opts.MissingProtoPolicy); p != MissingProtoPolicyStrict && p != MissingProtoPolicyWarn {
		return fmt.Errorf("invalid MissingProtoPolicy %q", opts.MissingProtoPolicy)
	}
	return nil
}

func normalizePolicy(v MissingProtoPolicy) MissingProtoPolicy {
	if v == "" {
		return MissingProtoPolicyStrict
	}
	return MissingProtoPolicy(strings.ToLower(string(v)))
}
