package entproto

import (
	"errors"
	"fmt"
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
	DefaultProtoPackageName = "entpb"
	IDFieldNumber           = 1
)

var (
	ErrSchemaSkipped   = errors.New("entproto: schema not annotated with Generate=true")
	repeatedFieldLabel = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
)

// LoadAdapter takes a *gen.Graph and parses it into protobuf file descriptors
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

// Adapter facilitates the transformation of ent gen.Type to desc.FileDescriptors
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

// AllFileDescriptors returns a file descriptor per proto package for each package that contains
// a successfully parsed ent.Schema. The map also includes stub descriptors for externally registered
// custom types; use GeneratedFileDescriptors to filter those out.
func (a *Adapter) AllFileDescriptors() map[string]*desc.FileDescriptor {
	return a.descriptors
}

// GeneratedFileDescriptors returns the file descriptors that entproto owns and
// should write to disk, excluding stubs synthesised for RegisterCustomType
// dependencies.
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

// GetMessageDescriptor retrieves the protobuf message descriptor for `schemaName`, if an error was returned
// while trying to parse that error they are returned
func (a *Adapter) GetMessageDescriptor(schemaName string) (*desc.MessageDescriptor, error) {
	fd, err := a.GetFileDescriptor(schemaName)
	if err != nil {
		return nil, err
	}
	findMessage := fd.FindMessage(fd.GetPackage() + "." + schemaName)
	if findMessage != nil {
		return findMessage, nil
	}
	return nil, errors.New("entproto: couldnt find message descriptor")
}

// parse transforms the ent gen.Type objects into file descriptors
func (a *Adapter) parse() error {
	var dpbDescriptors []*descriptorpb.FileDescriptorProto

	protoPackages := make(map[string]*descriptorpb.FileDescriptorProto)
	protoPackageDeps := make(map[string]map[string]struct{})
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

		if _, ok := protoPackages[protoPkg]; !ok {
			goPkg := a.goPackageName(protoPkg)
			protoPackages[protoPkg] = &descriptorpb.FileDescriptorProto{
				Name:    relFileName(protoPkg),
				Package: &protoPkg,
				Syntax:  toPtr("proto3"),
				Options: &descriptorpb.FileOptions{
					GoPackage: &goPkg,
				},
			}
			protoPackageDeps[protoPkg] = make(map[string]struct{})
		}
		fd := protoPackages[protoPkg]
		fd.MessageType = append(fd.MessageType, messageDescriptor)
		a.schemaProtoFiles[genType.Name] = *fd.Name

		depPaths, err := a.extractDepPaths(protoPkg, messageDescriptor, customStubs)
		if err != nil {
			a.errors[genType.Name] = err
			continue
		}
		for _, depPath := range depPaths {
			depSet, ok := protoPackageDeps[protoPkg]
			if !ok || depSet == nil {
				depSet = make(map[string]struct{})
				protoPackageDeps[protoPkg] = depSet
			}
			if _, seen := depSet[depPath]; seen {
				continue
			}
			depSet[depPath] = struct{}{}
			fd.Dependency = append(fd.Dependency, depPath)
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

// schemaErrorSummary renders the per-schema parse failures stored in a.errors.
func (a *Adapter) schemaErrorSummary() string {
	names := make([]string, 0, len(a.errors))
	for name := range a.errors {
		names = append(names, name)
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
	if f == nil || f.Annotations == nil {
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

// GetFileDescriptor returns the proto file descriptor containing the transformed proto message descriptor for
// `schemaName` along with any other messages in the same protobuf package.
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
	msgAnnot, err := extractMessageAnnotation(genType)
	if err != nil || !msgAnnot.Generate {
		return nil, ErrSchemaSkipped
	}
	msg := &descriptorpb.DescriptorProto{
		Name:     &genType.Name,
		EnumType: []*descriptorpb.EnumDescriptorProto(nil),
	}

	// The default (non-user-defined) ID field is implicitly field number 1.
	// Rather than mutating the caller's graph (genType.ID.Annotations), clone
	// the ID field and annotate the clone so repeated LoadAdapter calls are
	// side-effect free.
	idField := genType.ID
	if !genType.ID.UserDefined {
		annotations := genType.ID.Annotations
		hasFieldAnno := false
		if annotations != nil {
			_, hasFieldAnno = annotations[FieldAnnotation]
		}
		if !hasFieldAnno {
			clone := *genType.ID
			clone.Annotations = make(map[string]any, len(annotations)+1)
			for k, v := range annotations {
				clone.Annotations[k] = v
			}
			clone.Annotations[FieldAnnotation] = Field(IDFieldNumber)
			idField = &clone
		}
	}

	all := []*gen.Field{idField}
	all = append(all, genType.Fields...)

	for _, f := range all {
		if _, ok := f.Annotations[SkipAnnotation]; ok {
			continue
		}

		protoField, err := toProtoFieldDescriptor(f)
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
		if descriptor != nil {
			msg.Field = append(msg.Field, descriptor)
		}
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
	t := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	msgTypeName := pascal(e.Type.Name)

	edgeAnnotation, err := extractEdgeAnnotation(e)
	if err != nil {
		return nil, fmt.Errorf("entproto: failed extracting proto field number annotation: %w", err)
	}

	if edgeAnnotation.Number == 1 {
		return nil, fmt.Errorf("entproto: edge %q has number 1 which is reserved for id", e.Name)
	}

	if num := int64(edgeAnnotation.Number); num > math.MaxInt32 || num < math.MinInt32 {
		return nil, &FieldNumberOverflowError{
			Schema: source.Name,
			Field:  e.Name,
			Number: edgeAnnotation.Number,
		}
	}
	fieldNum := int32(edgeAnnotation.Number) //nolint:gosec
	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Number: &fieldNum,
		Name:   &e.Name,
		Type:   &t,
	}

	if !e.Unique {
		fieldDesc.Label = &repeatedFieldLabel
	}

	relType, ok := a.nodeByName[msgTypeName]
	if !ok {
		return nil, fmt.Errorf("entproto: could not find schema %q in graph", msgTypeName)
	}
	dstAnnotation, err := extractMessageAnnotation(relType)
	if err != nil || !dstAnnotation.Generate {
		return nil, fmt.Errorf("entproto: message %q is not generated", msgTypeName)
	}

	sourceAnnotation, err := extractMessageAnnotation(source)
	if err != nil {
		return nil, err
	}
	if sourceAnnotation.Package == dstAnnotation.Package {
		fieldDesc.TypeName = &msgTypeName
	} else {
		fqn := dstAnnotation.Package + "." + msgTypeName
		fieldDesc.TypeName = &fqn
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
	enumName := pascal(fld.Name)
	dp := &descriptorpb.EnumDescriptorProto{
		Name:  toPtr(enumName),
		Value: []*descriptorpb.EnumValueDescriptorProto{},
	}
	if !fld.Default {
		dp.Value = append(dp.Value, &descriptorpb.EnumValueDescriptorProto{
			Number: toPtr[int32](0),
			Name:   toPtr(strings.ToUpper(snake(fld.Name)) + "_UNSPECIFIED"),
		})
	}
	for _, opt := range fld.Enums {
		n := strings.ToUpper(snake(NormalizeEnumIdentifier(opt.Value)))
		if !enumAnnotation.OmitFieldPrefix {
			n = strings.ToUpper(snake(fld.Name)) + "_" + n
		}
		dp.Value = append(dp.Value, &descriptorpb.EnumValueDescriptorProto{
			Number: toPtr(enumAnnotation.Options[opt.Value]),
			Name:   toPtr(n),
		})
	}
	return dp, nil
}

func toProtoFieldDescriptor(f *gen.Field) (*descriptorpb.FieldDescriptorProto, error) {
	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Name: &f.Name,
	}
	fann, err := extractFieldAnnotation(f)
	if err != nil {
		return nil, err
	}
	if num := int64(fann.Number); num > math.MaxInt32 || num < math.MinInt32 {
		return nil, fmt.Errorf("value %v overflows int32", num)
	}
	fieldNumber := int32(fann.Number) //nolint:gosec
	if fieldNumber == 1 && strings.ToUpper(f.Name) != "ID" {
		return nil, fmt.Errorf("entproto: field %q has number 1 which is reserved for id", f.Name)
	}
	fieldDesc.Number = &fieldNumber
	if fann.Type != descriptorpb.FieldDescriptorProto_Type(0) {
		fieldDesc.Type = &fann.Type
		if len(fann.TypeName) > 0 {
			typeName := fann.TypeName
			if fann.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
				fann.Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
				typeName = normalizeCustomTypeName(typeName)
			}
			fieldDesc.TypeName = &typeName
			if fann.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && fann.ProtoFile != "" {
				if fann.ProtoPackage != "" && fann.MessagePath != "" {
					registerCustomTypeDetails(fann.TypeName, fann.ProtoFile, fann.ProtoPackage, fann.MessagePath)
				} else {
					registerCustomType(fann.TypeName, fann.ProtoFile)
				}
			}
		}
		return fieldDesc, nil
	}

	// Extract type details
	var pbType descriptorpb.FieldDescriptorProto_Type
	var msgName string
	var repeated bool

	if f.Type.Type == field.TypeJSON {
		switch f.Type.Ident {
		case "[]string":
			pbType = descriptorpb.FieldDescriptorProto_TYPE_STRING
			repeated = true
		case "[]int32":
			pbType = descriptorpb.FieldDescriptorProto_TYPE_INT32
			repeated = true
		case "[]int64":
			pbType = descriptorpb.FieldDescriptorProto_TYPE_INT64
			repeated = true
		case "[]uint32":
			pbType = descriptorpb.FieldDescriptorProto_TYPE_UINT32
			repeated = true
		case "[]uint64":
			pbType = descriptorpb.FieldDescriptorProto_TYPE_UINT64
			repeated = true
		default:
			return nil, fmt.Errorf("unsupported field type %q", f.Type.ConstName())
		}
	} else {
		cfg, ok := typeMap[f.Type.Type]
		if !ok || cfg.unsupported {
			return nil, fmt.Errorf("unsupported field type %q", f.Type.ConstName())
		}
		pbType = cfg.pbType
		msgName = cfg.msgTypeName
		if cfg.namer != nil {
			msgName = cfg.namer(f)
		}
	}

	fieldDesc.Type = &pbType
	if msgName != "" {
		fieldDesc.TypeName = &msgName
	}
	if repeated {
		fieldDesc.Label = &repeatedFieldLabel
	}
	return fieldDesc, nil
}

func toPtr[T any](t T) *T {
	return &t
}

func extractGenTypeByName(graph *gen.Graph, name string) (*gen.Type, error) {
	for _, sch := range graph.Nodes {
		if sch.Name == name {
			return sch, nil
		}
	}
	return nil, fmt.Errorf("entproto: could not find schema %q in graph", name)
}
