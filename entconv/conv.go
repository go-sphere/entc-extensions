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

// Options configures converter generation. Build it with [NewOptions] (or
// [DefaultOptions]) so unset fields get the scaffold defaults; a zero Options
// fails validation. Relative paths are resolved against the process working
// directory. Options is read, not modified, by the Generate functions.
type Options struct {
	// SchemaPath is the ent schema directory passed to entc.LoadGraph.
	// Required.
	SchemaPath string
	// EntPackagePath is the import path of the generated ent package, for
	// example "example.com/app/ent". Required.
	EntPackagePath string
	// IDType is the Go type of ent ID fields: "int", "int64", "uint",
	// "uint64", or "string"; empty means "int64". It must match the
	// gen.Config.IDType used for ent code generation. When the generated ent
	// package is found on disk, a mismatch fails with [*IDTypeMismatchError].
	IDType string
	// ProtoFile is the path of the .pb.go file that declares the protobuf
	// message structs and enum constants. Required.
	ProtoFile string
	// ConvPackage is the package name of the generated converter code.
	// Required.
	ConvPackage string
	// ProtoPackagePath is the import path of the package containing
	// ProtoFile. Required.
	ProtoPackagePath string
	// ProtoAlias is the import name used for ProtoPackagePath in generated
	// code. Empty means ConvPackage is used as the alias.
	ProtoAlias string
	// OutDir is the directory written by [GenerateConverterFile]. Required by
	// validation even for [GenerateConverter], which does not use it.
	OutDir string
	// MissingProtoPolicy decides what happens when an ent schema has no
	// message of the same name in ProtoFile. Empty means
	// [MissingProtoPolicyStrict]; values are compared case-insensitively.
	MissingProtoPolicy MissingProtoPolicy
	// WarningHandler receives the [*MissingProtoMessagesError] under
	// [MissingProtoPolicyWarn]. Nil means the warning is printed to standard
	// error. It is called synchronously at most once per generation.
	WarningHandler func(error)
}

// MissingProtoPolicy selects how generation treats ent schemas without a
// matching protobuf message; see [Options.MissingProtoPolicy].
type MissingProtoPolicy string

const (
	// MissingProtoPolicyStrict fails generation with a
	// [*MissingProtoMessagesError]. It is the default.
	MissingProtoPolicyStrict MissingProtoPolicy = "strict"
	// MissingProtoPolicyWarn reports the [*MissingProtoMessagesError] through
	// [Options.WarningHandler] and generates converters for the matched
	// schemas only.
	MissingProtoPolicyWarn MissingProtoPolicy = "warn"
)

// MissingProtoMessagesError lists ent schemas that have no protobuf message of
// the same name in [Options.ProtoFile]. errors.Is reports true for any target
// of this type, regardless of its contents.
type MissingProtoMessagesError struct {
	Missing []string // ent schema names, sorted
}

func (e *MissingProtoMessagesError) Error() string {
	return fmt.Sprintf("missing proto messages for ent types: %s", strings.Join(e.Missing, ", "))
}

func (e *MissingProtoMessagesError) Is(target error) bool {
	var t *MissingProtoMessagesError
	return errors.As(target, &t)
}

// Option modifies [Options]; see [NewOptions].
type Option func(*Options)

// DefaultOptions returns a new Options for the go-sphere scaffold layout,
// relative to the working directory and the main module path M of the running
// binary (empty when build information is unavailable):
//
//	SchemaPath:         "./internal/pkg/database/schema"
//	EntPackagePath:     M + "/internal/pkg/database/ent"
//	IDType:             "int64"
//	ProtoFile:          "./api/entpb/entpb.pb.go"
//	ConvPackage:        "entmap"
//	ProtoPackagePath:   M + "/api/entpb"
//	ProtoAlias:         "entpb"
//	OutDir:             "./internal/pkg/render/entmap"
//	MissingProtoPolicy: MissingProtoPolicyStrict
//
// The module-derived paths are correct when the generator is a main package of
// the same module, run with go run from the module root.
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

// NewOptions returns [DefaultOptions] with opts applied in order; nil options
// are ignored and later options override earlier ones.
func NewOptions(opts ...Option) *Options {
	o := DefaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// WithSchemaPath sets [Options.SchemaPath].
func WithSchemaPath(v string) Option {
	return func(o *Options) {
		o.SchemaPath = v
	}
}

// WithEntPackagePath sets [Options.EntPackagePath].
func WithEntPackagePath(v string) Option {
	return func(o *Options) {
		o.EntPackagePath = v
	}
}

// WithIDType sets [Options.IDType].
func WithIDType(v string) Option {
	return func(o *Options) {
		o.IDType = v
	}
}

// WithProtoFile sets [Options.ProtoFile].
func WithProtoFile(v string) Option {
	return func(o *Options) {
		o.ProtoFile = v
	}
}

// WithConvPackage sets [Options.ConvPackage].
func WithConvPackage(v string) Option {
	return func(o *Options) {
		o.ConvPackage = v
	}
}

// WithProtoPackagePath sets [Options.ProtoPackagePath].
func WithProtoPackagePath(v string) Option {
	return func(o *Options) {
		o.ProtoPackagePath = v
	}
}

// WithProtoAlias sets [Options.ProtoAlias].
func WithProtoAlias(v string) Option {
	return func(o *Options) {
		o.ProtoAlias = v
	}
}

// WithOutDir sets [Options.OutDir].
func WithOutDir(v string) Option {
	return func(o *Options) {
		o.OutDir = v
	}
}

// WithMissingProtoPolicy sets [Options.MissingProtoPolicy].
func WithMissingProtoPolicy(v MissingProtoPolicy) Option {
	return func(o *Options) {
		o.MissingProtoPolicy = v
	}
}

// WithWarningHandler sets [Options.WarningHandler].
func WithWarningHandler(h func(error)) Option {
	return func(o *Options) {
		o.WarningHandler = h
	}
}

// RequiredOptionError reports a required [Options] field that is empty. Field
// names the option: "ProtoFile", "SchemaPath", "EntPackage" (for
// EntPackagePath), "ProtoPackage" (for ConvPackage), "ProtoPackagePath", or
// "OutDir".
type RequiredOptionError struct {
	Field string
}

// InvalidIDTypeError reports an [Options.IDType] value that is not one of
// "int", "int64", "uint", "uint64", "string", or empty.
type InvalidIDTypeError struct {
	Value string
}

func (e *InvalidIDTypeError) Error() string {
	return fmt.Sprintf("unsupported IDType %q; valid values are int, int64, uint, uint64, and string", e.Value)
}

// ProtoFieldContractError reports that a message struct in [Options.ProtoFile]
// does not match what the ent schema and its entproto annotations imply:
// Field is missing (Actual is empty) or has Go type Actual instead of Expected.
// It usually means the .pb.go file is stale; regenerate it from the current
// .proto files.
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

// GenerateConverter returns the converter source for all matched schemas as a
// single gofmt-formatted file in package opts.ConvPackage. It writes no files;
// opts.OutDir must still be non-empty.
//
// GenerateConverter loads the ent schema (compiling it with go/packages), parses
// opts.ProtoFile, and validates both before generating. Errors include
// [*RequiredOptionError] and [*InvalidIDTypeError] for invalid options,
// [*IDTypeMismatchError], [*MissingProtoMessagesError] under
// [MissingProtoPolicyStrict], and, wrapped, [*ProtoFieldContractError] and
// [*EnumConstMissingError]; use errors.As to inspect them. It also fails when no
// schema matches a message. opts must not be nil.
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

// GenerateConverterWithOptions is [GenerateConverter] with [NewOptions](opts...).
func GenerateConverterWithOptions(opts ...Option) ([]byte, error) {
	return GenerateConverter(NewOptions(opts...))
}

// GenerateConverterFile validates like [GenerateConverter] and returns the
// same errors, then writes one file per matched schema, named after the
// lower-cased schema (user.go for User), into opts.OutDir, creating it if
// needed. Before writing, it deletes every .go file directly in OutDir whose
// content starts with "// Code generated by protoc-gen-entconv. DO NOT EDIT.",
// so converters for removed schemas disappear; other files are left alone.
// Writes are not atomic: an error after validation can leave OutDir partially
// updated. opts must not be nil.
func GenerateConverterFile(opts *Options) error {
	cg, err := prepareGenerator(opts)
	if err != nil {
		return err
	}
	return cg.GenerateAll(opts.OutDir)
}

// GenerateConverterFileWithOptions is [GenerateConverterFile] with
// [NewOptions](opts...). With no options it uses [DefaultOptions].
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
// converter code is written). It is used internally; no exported function
// returns it, and its Messages values have an internal type.
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
			fieldName := mapping.PbStructField()
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
			if err := validateExternalTypeEntShape(typeInfo, mapping, expected); err != nil {
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
// validateExternalTypeEntShape checks the ent side of fields the converter
// assigns directly: message fields (entproto.MessageField on a JSON column)
// and external enums (entproto.EnumField on a field whose GoType is the
// generated enum).
func validateExternalTypeEntShape(typeInfo generator.TypeInfo, mapping *entproto.FieldMappingDescriptor, pbGoType string) error {
	if mapping.IsEdgeField || mapping.EntField == nil {
		return nil
	}
	var kind string
	switch {
	case mapping.PbFieldDescriptor.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		kind = "message"
	case mapping.IsEnumField && !mapping.IsEntEnum():
		kind = "external enum"
		// A nillable ent field holds the same pointer protoc-gen-go uses for
		// the proto3 optional field, but ent reports the element type.
		if mapping.EntField.Nillable {
			pbGoType = strings.TrimPrefix(pbGoType, "*")
		}
	default:
		return nil
	}
	entType := mapping.EntField.Type.String()
	if entType == "" {
		return nil
	}
	// ent JSON columns and GoType fields carry the Go type in Ident (e.g.
	// *timestamppb.Timestamp); basic scalar columns have no Go type and cannot
	// hold a message or an external enum.
	if canonicalProtoGoType(entType) != canonicalProtoGoType(pbGoType) {
		return &ProtoFieldContractError{
			Message: typeInfo.MessageName,
			Field:   mapping.PbFieldDescriptor.GetName(),
			Expected: fmt.Sprintf(
				"ent field of Go type compatible with %s (%s field; converter assigns it directly)",
				pbGoType, kind,
			),
			Actual: entType,
		}
	}
	return nil
}

// EnumConstMissingError reports that the generated converter would reference a
// protobuf enum constant that does not exist in the supplied .pb.go file. When
// the constant name itself cannot be resolved, Const holds that error's text
// instead of a name.
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
			if entField == nil || !mapping.IsEntEnum() {
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
