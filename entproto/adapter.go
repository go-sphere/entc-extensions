package entproto

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/jhump/protoreflect/desc"         //nolint:staticcheck
	"github.com/jhump/protoreflect/desc/builder" //nolint:staticcheck
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	// DefaultProtoPackageName is the protobuf package used when a [Message]
	// annotation sets no [PackageName].
	DefaultProtoPackageName = "entpb"
	// IDFieldNumber is the protobuf field number reserved for the ID field.
	IDFieldNumber = 1

	maxFieldNumber           = 1<<29 - 1
	reservedFieldNumberStart = 19000
	reservedFieldNumberEnd   = 19999
)

// ErrSchemaSkipped is returned by [Adapter.GetFileDescriptor] and
// [Adapter.GetMessageDescriptor] for a schema that has no [Message]
// annotation or is annotated with [SkipGen].
var ErrSchemaSkipped = errors.New("entproto: schema not annotated with Generate=true")

// LoadAdapter converts every schema in graph into protobuf descriptors without
// writing files. It reads annotations as they are; call [FixGraph] first to get
// [WithAutoFill] behavior. Neither the graph nor its annotations are modified,
// but [MessageField] annotations register their types in the process-wide
// [RegisterCustomType] registry.
//
// Per-schema conversion failures do not fail LoadAdapter; they are reported
// later by [Adapter.GetFileDescriptor] for that schema. LoadAdapter returns an
// error when descriptors cannot be linked, including a [*DanglingReferenceError]
// when a converted schema references one that failed.
func LoadAdapter(graph *gen.Graph) (*Adapter, error) {
	a := &Adapter{
		graph:            graph,
		nodeByName:       make(map[string]*gen.Type, len(graph.Nodes)),
		protoPkgByType:   make(map[string]string, len(graph.Nodes)),
		descriptors:      make(map[string]*desc.FileDescriptor),
		schemaProtoFiles: make(map[string]string),
		externalFiles:    make(map[string]struct{}),
		errors:           make(map[string]error),
	}
	for _, node := range graph.Nodes {
		a.nodeByName[node.Name] = node
	}
	if err := a.parse(); err != nil {
		return nil, err
	}
	return a, nil
}

// Adapter holds the protobuf descriptors built from an ent graph by
// [LoadAdapter]. It is not modified after LoadAdapter returns. Maps and
// descriptors returned by its methods are shared with the Adapter and must not
// be modified by callers.
type Adapter struct {
	graph            *gen.Graph
	nodeByName       map[string]*gen.Type
	protoPkgByType   map[string]string
	descriptors      map[string]*desc.FileDescriptor
	schemaProtoFiles map[string]string
	// externalFiles holds proto file paths that were synthesised purely to
	// satisfy cross-file linking for RegisterCustomType references. They must
	// not be written to disk.
	externalFiles map[string]struct{}
	errors        map[string]error
}

// AllFileDescriptors returns the file descriptors keyed by .proto file name
// (for example "entpb/entpb.proto"): one per protobuf package that contains a
// successfully converted schema, plus stub descriptors for externally
// registered custom types. Use [Adapter.GeneratedFileDescriptors] to exclude
// the stubs.
func (a *Adapter) AllFileDescriptors() map[string]*desc.FileDescriptor {
	return a.descriptors
}

// GeneratedFileDescriptors returns the file descriptors that entproto owns and
// writes to disk, keyed by .proto file name, excluding stubs synthesised for
// [RegisterCustomType] dependencies.
func (a *Adapter) GeneratedFileDescriptors() map[string]*desc.FileDescriptor {
	if len(a.externalFiles) == 0 {
		return a.descriptors
	}
	out := make(map[string]*desc.FileDescriptor, len(a.descriptors))
	for name, fd := range a.descriptors {
		if _, ext := a.externalFiles[name]; ext {
			continue
		}
		out[name] = fd
	}
	return out
}

// GetMessageDescriptor returns the protobuf message descriptor for the schema
// named schemaName (the ent type name, for example "User"). It returns the
// schema's conversion error if it failed, [ErrSchemaSkipped] if it is not
// generated, or an error if the schema is unknown.
func (a *Adapter) GetMessageDescriptor(schemaName string) (*desc.MessageDescriptor, error) {
	fd, err := a.GetFileDescriptor(schemaName)
	if err != nil {
		return nil, err
	}
	findMessage := fd.FindMessage(fd.GetPackage() + "." + schemaName)
	if findMessage != nil {
		return findMessage, nil
	}
	return nil, fmt.Errorf("entproto: could not find message descriptor for schema %s", schemaName)
}

// parse transforms the ent gen.Type objects into file descriptors
func (a *Adapter) parse() error {
	var dpbDescriptors []*descriptorpb.FileDescriptorProto

	protoPackages := make(map[string]*descriptorpb.FileDescriptorProto)
	customStubs := map[string]*descriptorpb.FileDescriptorProto{}

	for _, genType := range a.graph.Nodes {
		messageDescriptor, err := a.toProtoMessageDescriptor(genType)

		// store specific message parse failures
		if err != nil {
			a.errors[genType.Name] = err
			continue
		}

		protoPkg, err := a.protoPackageName(genType)
		if err != nil {
			a.errors[genType.Name] = err
			continue
		}

		// Resolve dependencies before adding the message to its file, so a
		// schema that fails here stays a per-schema error instead of leaving an
		// unresolvable message behind for the linker.
		depPaths, err := a.extractDepPaths(protoPkg, messageDescriptor, customStubs)
		if err != nil {
			a.errors[genType.Name] = err
			continue
		}

		if _, ok := protoPackages[protoPkg]; !ok {
			goPkg := a.goPackageName(protoPkg)
			protoPackages[protoPkg] = &descriptorpb.FileDescriptorProto{
				Name:    relFileName(protoPkg),
				Package: toPtr(protoPkg),
				Syntax:  toPtr("proto3"),
				Options: &descriptorpb.FileOptions{
					GoPackage: &goPkg,
				},
			}
		}
		fd := protoPackages[protoPkg]
		fd.MessageType = append(fd.MessageType, messageDescriptor)
		a.schemaProtoFiles[genType.Name] = fd.GetName()

		for _, depPath := range depPaths {
			if !slices.Contains(fd.Dependency, depPath) {
				fd.Dependency = append(fd.Dependency, depPath)
			}
		}
	}

	for _, fd := range protoPackages {
		dpbDescriptors = append(dpbDescriptors, fd)
	}
	for _, stub := range customStubs {
		dpbDescriptors = append(dpbDescriptors, stub)
		a.externalFiles[stub.GetName()] = struct{}{}
	}

	// If any schema failed to parse, fail before linking: a successful message
	// may reference a failed one (e.g. via an edge or a message field), which
	// would otherwise surface as a misleading "cannot resolve type" error deep
	// inside desc.CreateFileDescriptors, hiding the actual root cause.
	if len(a.errors) > 0 {
		if err := a.linkDependencyError(); err != nil {
			return err
		}
	}

	descriptors, err := desc.CreateFileDescriptors(dpbDescriptors)
	if err != nil {
		if len(a.errors) > 0 {
			return fmt.Errorf("%w (schema errors: %v)", err, a.schemaErrorSummary())
		}
		return err
	}

	for dp, fd := range descriptors {
		fbuild, err := builder.FromFile(fd)
		if err != nil {
			return err
		}
		fbuild.SetSyntaxComments(builder.Comments{
			LeadingComment: " Code generated by entproto. DO NOT EDIT.",
		})
		fd, err = fbuild.Build()
		if err != nil {
			return err
		}
		descriptors[dp] = fd
	}

	a.descriptors = descriptors

	return nil
}

func (a *Adapter) goPackageName(protoPkgName string) string {
	// TODO(rotemtam): make this configurable from an annotation
	entBase := a.graph.Package
	slashed := strings.ReplaceAll(protoPkgName, ".", "/")
	return path.Join(entBase, "proto", slashed)
}

// schemaErrorSummary renders the per-schema parse failures stored in a.errors,
// leaving out schemas that are merely not generated.
func (a *Adapter) schemaErrorSummary() string {
	names := make([]string, 0, len(a.errors))
	for name, err := range a.errors {
		if !errors.Is(err, ErrSchemaSkipped) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s: %v", name, a.errors[name])
	}
	return b.String()
}

// linkDependencyError reports when a successfully parsed schema references a
// schema that failed to parse (e.g. through an edge or a message-typed field).
// Returning an error here surfaces the root cause (the failed schema) together
// with the referencing field, instead of letting desc.CreateFileDescriptors fail
// later on a dangling type name with no relation to the actual problem.
func (a *Adapter) linkDependencyError() error {
	for _, genType := range a.graph.Nodes {
		if _, failed := a.errors[genType.Name]; failed {
			continue
		}
		for _, e := range genType.Edges {
			if _, skip := e.Annotations[SkipAnnotation]; skip {
				continue
			}
			if _, failed := a.errors[e.Type.Name]; failed {
				return &DanglingReferenceError{
					Schema:    genType.Name,
					Field:     e.Name,
					RefSchema: e.Type.Name,
					Cause:     a.errors[e.Type.Name],
				}
			}
		}
		for _, f := range genType.Fields {
			ref := a.fieldReferencedSchema(f)
			if ref == "" {
				continue
			}
			if _, failed := a.errors[ref]; failed {
				return &DanglingReferenceError{
					Schema:    genType.Name,
					Field:     f.Name,
					RefSchema: ref,
					Cause:     a.errors[ref],
				}
			}
		}
	}
	return nil
}

// fieldReferencedSchema returns the name of a schema referenced by a
// message/enum-typed field annotation, or "" when the field does not reference a
// graph schema. External custom types registered via RegisterCustomType are not
// graph schemas and are ignored.
func (a *Adapter) fieldReferencedSchema(f *gen.Field) string {
	if f == nil {
		return ""
	}
	if _, skip := f.Annotations[SkipAnnotation]; skip {
		return ""
	}
	if _, ok := f.Annotations[FieldAnnotation]; !ok {
		return ""
	}
	ann, err := extractFieldAnnotation(f)
	if err != nil || ann.TypeName == "" {
		return ""
	}
	if ann.Type != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE &&
		ann.Type != descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		return ""
	}
	name := protoTypeShortName(ann.TypeName)
	if _, ok := a.nodeByName[name]; ok {
		return name
	}
	return ""
}

// GetFileDescriptor returns the file descriptor that contains the message for
// the schema named schemaName, along with every other message in the same
// protobuf package. If the schema could not be converted, it returns that
// conversion error unchanged (for example a [*DuplicateFieldNumberError] or
// [ErrSchemaSkipped]); an unknown schema name yields a plain error.
func (a *Adapter) GetFileDescriptor(schemaName string) (*desc.FileDescriptor, error) {
	if err, ok := a.errors[schemaName]; ok {
		return nil, err
	}
	fn, ok := a.schemaProtoFiles[schemaName]
	if !ok {
		return nil, fmt.Errorf("entproto: could not find file descriptor for schema %s", schemaName)
	}

	dsc, ok := a.descriptors[fn]
	if !ok {
		return nil, fmt.Errorf("entproto: could not find file descriptor for schema %s", schemaName)
	}

	return dsc, nil
}

func protoPackageName(genType *gen.Type) (string, error) {
	msgAnnot, err := extractMessageAnnotation(genType)
	if err != nil {
		return "", err
	}

	if msgAnnot.Package != "" {
		return msgAnnot.Package, nil
	}
	return DefaultProtoPackageName, nil
}

func (a *Adapter) protoPackageName(genType *gen.Type) (string, error) {
	if pkg, ok := a.protoPkgByType[genType.Name]; ok {
		return pkg, nil
	}
	pkg, err := protoPackageName(genType)
	if err != nil {
		return "", err
	}
	a.protoPkgByType[genType.Name] = pkg
	return pkg, nil
}

func relFileName(packageName string) *string {
	parts := strings.Split(packageName, ".")
	fileName := parts[len(parts)-1] + ".proto"
	parts = append(parts, fileName)
	joined := filepath.Join(parts...)
	return &joined
}

func (a *Adapter) extractDepPaths(selfPackageName string, m *descriptorpb.DescriptorProto, customStubs map[string]*descriptorpb.FileDescriptorProto) ([]string, error) {
	var out []string
	for _, fld := range m.Field {
		if fld.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			continue
		}
		fieldTypeName := fld.GetTypeName()
		if entry, ok := lookupCustomType(fieldTypeName); ok {
			if err := addCustomTypeToStub(customStubs, entry); err != nil {
				return nil, err
			}
			out = append(out, entry.ProtoFile)
			continue
		}
		depTypeName := protoTypeShortName(fieldTypeName)
		depType, ok := a.nodeByName[depTypeName]
		if !ok {
			return nil, fmt.Errorf("entproto: failed extracting deps, unknown path for %s", fieldTypeName)
		}
		depPackageName, err := a.protoPackageName(depType)
		if err != nil {
			return nil, err
		}
		if depPackageName != selfPackageName {
			importPath := relFileName(depPackageName)
			out = append(out, *importPath)
		}
	}
	return out, nil
}

func protoTypeShortName(typeName string) string {
	parts := strings.Split(typeName, ".")
	return parts[len(parts)-1]
}

func (a *Adapter) toProtoMessageDescriptor(genType *gen.Type) (*descriptorpb.DescriptorProto, error) {
	if genType.Annotations[MessageAnnotation] == nil {
		return nil, ErrSchemaSkipped
	}
	msgAnnot, err := extractMessageAnnotation(genType)
	if err != nil {
		return nil, err
	}
	if !msgAnnot.Generate {
		return nil, ErrSchemaSkipped
	}
	msg := &descriptorpb.DescriptorProto{
		Name: toPtr(genType.Name),
	}

	// The default (non-user-defined) ID field is implicitly field number 1.
	// Rather than mutating the caller's graph (genType.ID.Annotations), clone
	// the ID field and annotate the clone so repeated LoadAdapter calls are
	// side-effect free.
	idField := genType.ID
	if _, annotated := genType.ID.Annotations[FieldAnnotation]; !genType.ID.UserDefined && !annotated {
		clone := *genType.ID
		clone.Annotations = make(map[string]any, len(genType.ID.Annotations)+1)
		maps.Copy(clone.Annotations, genType.ID.Annotations)
		clone.Annotations[FieldAnnotation] = Field(IDFieldNumber)
		idField = &clone
	}

	all := []*gen.Field{idField}
	all = append(all, genType.Fields...)

	for _, f := range all {
		if _, ok := f.Annotations[SkipAnnotation]; ok {
			continue
		}

		protoField, err := toProtoFieldDescriptor(genType.Name, f, f == idField)
		if err != nil {
			return nil, err
		}
		if f.Nillable && protoField.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED &&
			protoField.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			oneofIndex := int32(len(msg.OneofDecl)) //nolint:gosec // bounded by the number of message fields.
			protoField.Proto3Optional = toPtr(true)
			protoField.OneofIndex = &oneofIndex
			msg.OneofDecl = append(msg.OneofDecl, &descriptorpb.OneofDescriptorProto{Name: toPtr("_" + f.Name)})
		}
		// If the field is an enum type, we need to create the enum descriptor as well.
		if f.Type.Type == field.TypeEnum {
			dp, err := toProtoEnumDescriptor(f)
			if err != nil {
				return nil, err
			}
			msg.EnumType = append(msg.EnumType, dp)
		}
		msg.Field = append(msg.Field, protoField)
	}

	for _, e := range genType.Edges {
		if _, ok := e.Annotations[SkipAnnotation]; ok {
			continue
		}

		descriptor, err := a.extractEdgeFieldDescriptor(genType, e)
		if err != nil {
			return nil, err
		}
		msg.Field = append(msg.Field, descriptor)
	}

	// Verify no duplicate field numbers
	seen := make(map[int32]struct{})
	for _, fld := range msg.Field {
		if _, duplicate := seen[fld.GetNumber()]; duplicate {
			return nil, &DuplicateFieldNumberError{
				Message: msg.GetName(),
				Number:  fld.GetNumber(),
			}
		}
		seen[fld.GetNumber()] = struct{}{}
	}

	return msg, nil
}

func (a *Adapter) extractEdgeFieldDescriptor(source *gen.Type, e *gen.Edge) (*descriptorpb.FieldDescriptorProto, error) {
	edgeAnnotation, err := extractEdgeAnnotation(e)
	if err != nil {
		return nil, fmt.Errorf("entproto: failed extracting proto field number annotation: %w", err)
	}
	fieldNum, err := checkFieldNumber(source.Name, "edge", e.Name, edgeAnnotation.Number, false)
	if err != nil {
		return nil, err
	}

	target := e.Type
	dstAnnotation, err := extractMessageAnnotation(target)
	if err != nil || !dstAnnotation.Generate {
		return nil, fmt.Errorf("entproto: edge %q references schema %q, which is not generated; annotate the edge with entproto.Skip()",
			e.Name, target.Name)
	}
	sourcePkg, err := a.protoPackageName(source)
	if err != nil {
		return nil, err
	}
	targetPkg, err := a.protoPackageName(target)
	if err != nil {
		return nil, err
	}
	typeName := target.Name
	if sourcePkg != targetPkg {
		typeName = "." + targetPkg + "." + target.Name
	}

	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Number:   &fieldNum,
		Name:     toPtr(e.Name),
		Type:     toPtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
		TypeName: &typeName,
	}
	if !e.Unique {
		fieldDesc.Label = toPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED)
	}
	return fieldDesc, nil
}

func toProtoEnumDescriptor(fld *gen.Field) (*descriptorpb.EnumDescriptorProto, error) {
	enumAnnotation, err := extractEnumAnnotation(fld)
	if err != nil {
		return nil, err
	}
	if err := enumAnnotation.Verify(fld); err != nil {
		return nil, err
	}
	dp := &descriptorpb.EnumDescriptorProto{
		Name:  toPtr(pascal(fld.Name)),
		Value: make([]*descriptorpb.EnumValueDescriptorProto, 0, len(fld.Enums)+1),
	}
	if !fld.Default {
		dp.Value = append(dp.Value, &descriptorpb.EnumValueDescriptorProto{
			Number: toPtr[int32](0),
			Name:   toPtr(unspecifiedEnumValueName(fld)),
		})
	}
	for _, opt := range fld.Enums {
		dp.Value = append(dp.Value, &descriptorpb.EnumValueDescriptorProto{
			Number: toPtr(enumAnnotation.Options[opt.Value]),
			Name:   toPtr(enumAnnotation.protoValueName(fld, opt.Value)),
		})
	}
	return dp, nil
}

func toProtoFieldDescriptor(schemaName string, f *gen.Field, isID bool) (*descriptorpb.FieldDescriptorProto, error) {
	fann, err := extractFieldAnnotation(f)
	if err != nil {
		return nil, err
	}
	fieldNumber, err := checkFieldNumber(schemaName, "field", f.Name, fann.Number, isID)
	if err != nil {
		return nil, err
	}
	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Name:   toPtr(f.Name),
		Number: &fieldNumber,
	}
	if fann.Type != descriptorpb.FieldDescriptorProto_Type(0) {
		isReference := fann.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
			fann.Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM
		if isReference && fann.TypeName == "" {
			return nil, fmt.Errorf("entproto: field %q uses entproto.Type(%s) without entproto.TypeName", f.Name, fann.Type)
		}
		fieldDesc.Type = toPtr(fann.Type)
		if fann.TypeName != "" {
			typeName := fann.TypeName
			if isReference {
				typeName = normalizeCustomTypeName(typeName)
			}
			fieldDesc.TypeName = &typeName
			if fann.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && fann.ProtoFile != "" {
				registerCustomTypeDetails(fann.TypeName, fann.ProtoFile, fann.ProtoPackage, fann.MessagePath)
			}
		}
		return fieldDesc, nil
	}

	if f.Type.Type == field.TypeJSON {
		elem, ok := repeatedJSONTypes[f.Type.Ident]
		if !ok {
			return nil, unsupportedFieldTypeError(f)
		}
		fieldDesc.Type = toPtr(elem)
		fieldDesc.Label = toPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED)
		return fieldDesc, nil
	}

	cfg, ok := typeMap[f.Type.Type]
	if !ok || cfg.unsupported {
		return nil, unsupportedFieldTypeError(f)
	}
	fieldDesc.Type = toPtr(cfg.pbType)
	if cfg.namer != nil {
		fieldDesc.TypeName = toPtr(cfg.namer(f))
	}
	return fieldDesc, nil
}

func unsupportedFieldTypeError(f *gen.Field) error {
	return fmt.Errorf("entproto: field %q has unsupported type %s (%s); set entproto.Type or entproto.MessageField, or annotate it with entproto.Skip()",
		f.Name, f.Type.ConstName(), f.Type)
}

// checkFieldNumber validates n as the protobuf number of the field or edge
// (kind) named member in schema and returns it as an int32.
func checkFieldNumber(schema, kind, member string, n int, isID bool) (int32, error) {
	if int64(n) > math.MaxInt32 || int64(n) < math.MinInt32 {
		return 0, &FieldNumberOverflowError{Schema: schema, Field: member, Number: n}
	}
	switch {
	case n == IDFieldNumber && !isID:
		return 0, fmt.Errorf("entproto: %s %q has number 1 which is reserved for id", kind, member)
	case n < 1 || n > maxFieldNumber:
		return 0, fmt.Errorf("entproto: %s %q of schema %q has number %d, outside the protobuf range 1-%d",
			kind, member, schema, n, maxFieldNumber)
	case n >= reservedFieldNumberStart && n <= reservedFieldNumberEnd:
		return 0, fmt.Errorf("entproto: %s %q of schema %q has number %d, inside the range %d-%d reserved by protobuf",
			kind, member, schema, n, reservedFieldNumberStart, reservedFieldNumberEnd)
	}
	return int32(n), nil
}

func toPtr[T any](t T) *T {
	return &t
}
