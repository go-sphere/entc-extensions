package entconv

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"runtime/debug"
	"slices"
	"strings"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entconv/internal/converter"
	"github.com/go-sphere/entc-extensions/entconv/internal/generator"
	"github.com/go-sphere/entc-extensions/entconv/internal/pkgutil"
	"github.com/go-sphere/entc-extensions/entproto"
	"golang.org/x/tools/imports"
	"google.golang.org/protobuf/types/descriptorpb"
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

type InvalidIDTypeError struct {
	Value string
}

func (e *InvalidIDTypeError) Error() string {
	return fmt.Sprintf("unsupported IDType %q; valid values are int, int64, uint, uint64, and string", e.Value)
}

type ProtoFieldContractError struct {
	Message  string
	Field    string
	Expected string
	Actual   string
}

func (e *ProtoFieldContractError) Error() string {
	if e.Actual == "" {
		return fmt.Sprintf("proto message %s is missing field %s (expected Go type %s)", e.Message, e.Field, e.Expected)
	}
	return fmt.Sprintf("proto message %s field %s has Go type %s; expected %s", e.Message, e.Field, e.Actual, e.Expected)
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

	idType, err := parseIDType(opts.IDType)
	if err != nil {
		return nil, err
	}
	g, err := loadEntGraph(opts.SchemaPath, entPkg, idType)
	if err != nil {
		return nil, fmt.Errorf("loading ent graph: %w", err)
	}
	// Fail fast when the IDType option diverges from the real generated ent
	// structs (when they are available on disk). Without this check the
	// generator can emit converters that do not compile.
	if err := verifyIDTypesAgainstSource(g, opts.SchemaPath, entPkg); err != nil {
		return nil, err
	}

	protoTypes, err := parseProtoFile(opts.ProtoFile)
	if err != nil {
		return nil, fmt.Errorf("parsing proto file: %w", err)
	}

	typesToGenerate, missing := matchTypes(g, protoTypes.Messages)
	if missing != nil {
		switch normalizePolicy(opts.MissingProtoPolicy) {
		case MissingProtoPolicyWarn:
			if opts.WarningHandler != nil {
				opts.WarningHandler(missing)
			} else {
				// Without a caller-provided handler, surface the warning on
				// stderr instead of silently dropping the missing types.
				fmt.Fprintln(os.Stderr, "entconv:", missing)
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
	if err := validateProtoContracts(typesToGenerate, adapter); err != nil {
		return nil, fmt.Errorf("validating proto contract: %w", err)
	}
	if err := validateEnumConstReferences(typesToGenerate, adapter, protoTypes.Consts); err != nil {
		return nil, fmt.Errorf("validating enum constant references: %w", err)
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

// ProtoFileInfo holds the parsed contents of a generated .pb.go file: the
// message structs (used for field-contract validation) and the set of declared
// top-level constants (used to validate generated enum references before the
// converter code is written).
type ProtoFileInfo struct {
	Messages map[string]*generator.ProtoMessage
	Consts   map[string]struct{}
}

func parseProtoFile(filePath string) (*ProtoFileInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	info := &ProtoFileInfo{
		Messages: make(map[string]*generator.ProtoMessage),
		Consts:   make(map[string]struct{}),
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch spec := n.(type) {
		case *ast.TypeSpec:
			structType, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			msg := &generator.ProtoMessage{
				Name:   spec.Name.Name,
				Fields: parseStructFields(structType),
			}
			info.Messages[msg.Name] = msg
		case *ast.ValueSpec:
			// Top-level const declarations (enum values) declare exactly one
			// name per spec; collect them all.
			for _, name := range spec.Names {
				info.Consts[name.Name] = struct{}{}
			}
		}
		return true
	})

	return info, nil
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

func parseIDType(idType string) (*field.TypeInfo, error) {
	switch idType {
	case "int":
		return &field.TypeInfo{Type: field.TypeInt}, nil
	case "", "int64":
		return &field.TypeInfo{Type: field.TypeInt64}, nil
	case "uint":
		return &field.TypeInfo{Type: field.TypeUint}, nil
	case "uint64":
		return &field.TypeInfo{Type: field.TypeUint64}, nil
	case "string":
		return &field.TypeInfo{Type: field.TypeString}, nil
	default:
		return nil, &InvalidIDTypeError{Value: idType}
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
	if _, err := parseIDType(opts.IDType); err != nil {
		return err
	}
	if p := normalizePolicy(opts.MissingProtoPolicy); p != MissingProtoPolicyStrict && p != MissingProtoPolicyWarn {
		return fmt.Errorf("invalid MissingProtoPolicy %q", opts.MissingProtoPolicy)
	}
	return nil
}

func validateProtoContracts(types []generator.TypeInfo, adapter *entproto.Adapter) error {
	for _, typeInfo := range types {
		fieldMap, err := adapter.FieldMap(typeInfo.Type.Name)
		if err != nil {
			return err
		}
		actualFields := make(map[string]string, len(typeInfo.Message.Fields))
		for _, protoField := range typeInfo.Message.Fields {
			actualFields[protoField.Name] = protoField.Type
		}
		for _, mapping := range fieldMap.Fields() {
			fieldName := mapping.PbFieldName()
			expected := expectedProtoGoType(mapping)
			actual, ok := actualFields[fieldName]
			if !ok {
				return &ProtoFieldContractError{Message: typeInfo.MessageName, Field: fieldName, Expected: expected}
			}
			if canonicalProtoGoType(actual) != canonicalProtoGoType(expected) {
				return &ProtoFieldContractError{
					Message: typeInfo.MessageName, Field: fieldName, Expected: expected, Actual: actual,
				}
			}
			if err := validateMessageFieldEntShape(typeInfo, mapping, expected); err != nil {
				return err
			}
			// Fail before any file is written when a field's custom Go type has
			// no template-renderable conversion; otherwise the generator would
			// emit an identity assignment that does not compile.
			if _, err := converter.NewConverter(mapping, typeInfo.Type.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func expectedProtoGoType(mapping *entproto.FieldMappingDescriptor) string {
	fieldDescriptor := mapping.PbFieldDescriptor
	var name string
	switch fieldDescriptor.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		name = "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		name = "string"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		name = "[]byte"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		name = "int32"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		name = "int64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		name = "uint32"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		name = "uint64"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		name = "float32"
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		name = "float64"
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		enum := fieldDescriptor.GetEnumType()
		name = protoDescriptorGoName(enum.GetFullyQualifiedName(), enum.GetFile().GetPackage())
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		message := fieldDescriptor.GetMessageType()
		name = protoDescriptorGoName(message.GetFullyQualifiedName(), message.GetFile().GetPackage())
	}
	if fieldDescriptor.IsRepeated() && name != "[]byte" {
		if fieldDescriptor.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			name = "[]*" + name
		} else {
			name = "[]" + name
		}
	} else if fieldDescriptor.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		name = "*" + name
	} else if fieldDescriptor.AsFieldDescriptorProto().GetProto3Optional() && name != "[]byte" {
		name = "*" + name
	}
	return name
}

func protoDescriptorGoName(fullName, packageName string) string {
	name := strings.TrimPrefix(fullName, packageName+".")
	return strings.ReplaceAll(name, ".", "_")
}

func canonicalProtoGoType(typeName string) string {
	prefix := ""
	for {
		switch {
		case strings.HasPrefix(typeName, "[]"):
			prefix += "[]"
			typeName = strings.TrimPrefix(typeName, "[]")
		case strings.HasPrefix(typeName, "*"):
			prefix += "*"
			typeName = strings.TrimPrefix(typeName, "*")
		default:
			if index := strings.LastIndex(typeName, "."); index >= 0 {
				typeName = typeName[index+1:]
			}
			return prefix + typeName
		}
	}
}

func normalizePolicy(v MissingProtoPolicy) MissingProtoPolicy {
	if v == "" {
		return MissingProtoPolicyStrict
	}
	return MissingProtoPolicy(strings.ToLower(string(v)))
}

// validateMessageFieldEntShape checks that when a proto field is a message
// reference, the corresponding ent field holds a Go value that the converter
// can assign directly (the generator emits an identity assignment for these).
// The ent field is typically a JSON column typed as the same generated Go
// message. A mismatch here would silently produce code that does not compile.
func validateMessageFieldEntShape(typeInfo generator.TypeInfo, mapping *entproto.FieldMappingDescriptor, pbGoType string) error {
	if mapping.PbFieldDescriptor.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
		mapping.IsEdgeField || mapping.EntField == nil {
		return nil
	}
	entType := mapping.EntField.Type.String()
	if entType == "" {
		return nil
	}
	// ent JSON columns carry the Go type in Ident (e.g. *timestamppb.Timestamp);
	// basic scalar columns have no Go type and cannot hold a message.
	if canonicalProtoGoType(entType) != canonicalProtoGoType(pbGoType) {
		return &ProtoFieldContractError{
			Message: typeInfo.MessageName,
			Field:   mapping.PbFieldDescriptor.GetName(),
			Expected: fmt.Sprintf(
				"ent field of Go type compatible with %s (message field; converter assigns it directly)",
				pbGoType,
			),
			Actual: entType,
		}
	}
	return nil
}

// EnumConstMissingError reports that the generated converter would reference a
// protobuf enum constant that does not exist in the supplied .pb.go file.
type EnumConstMissingError struct {
	Message string
	Field   string
	Const   string
}

func (e *EnumConstMissingError) Error() string {
	return fmt.Sprintf(
		"proto message %s field %s references enum constant %s which is not declared in the .pb.go file",
		e.Message, e.Field, e.Const,
	)
}

// validateEnumConstReferences verifies that every protobuf enum constant the
// converter template will reference actually exists in the parsed .pb.go file.
// The constant name is resolved from the real protobuf enum descriptor (via the
// entproto.Enum number annotation), so this cannot diverge from protoc-gen-go's
// output the way a re-derived name would.
func validateEnumConstReferences(types []generator.TypeInfo, adapter *entproto.Adapter, consts map[string]struct{}) error {
	for _, typeInfo := range types {
		fieldMap, err := adapter.FieldMap(typeInfo.Type.Name)
		if err != nil {
			return err
		}
		for _, mapping := range fieldMap.Enums() {
			entField := mapping.EntField
			if entField == nil {
				continue
			}
			for _, opt := range entField.Enums {
				constName, err := mapping.PbEnumValueConstName(opt.Value)
				if err != nil {
					return &EnumConstMissingError{
						Message: typeInfo.MessageName,
						Field:   mapping.PbFieldDescriptor.GetName(),
						Const:   err.Error(),
					}
				}
				if _, ok := consts[constName]; !ok {
					return &EnumConstMissingError{
						Message: typeInfo.MessageName,
						Field:   mapping.PbFieldDescriptor.GetName(),
						Const:   constName,
					}
				}
			}
		}
	}
	return nil
}
