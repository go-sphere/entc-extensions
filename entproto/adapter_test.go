package entproto

import (
	"errors"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestToProtoMessageDescriptor_PreservesExistingIDAnnotations(t *testing.T) {
	id := &gen.Field{
		Name:        "id",
		Type:        &field.TypeInfo{Type: field.TypeInt},
		UserDefined: false,
		Annotations: map[string]any{"ExistingKey": "keep-me"},
	}
	node := &gen.Type{
		Name:        "Demo",
		ID:          id,
		Fields:      []*gen.Field{},
		Edges:       []*gen.Edge{},
		Annotations: map[string]any{MessageAnnotation: Message()},
	}
	a := &Adapter{}

	message, err := a.toProtoMessageDescriptor(node)
	if err != nil {
		t.Fatalf("toProtoMessageDescriptor returned error: %v", err)
	}
	if got := id.Annotations["ExistingKey"]; got != "keep-me" {
		t.Fatalf("existing ID annotation lost, got=%v", got)
	}
	if _, ok := id.Annotations[FieldAnnotation]; ok {
		t.Fatal("default ID field annotation must not be written back to the input graph")
	}
	if len(message.Field) == 0 || message.Field[0].GetNumber() != IDFieldNumber {
		t.Fatalf("generated ID field number = %v, want %d", message.Field[0].GetNumber(), IDFieldNumber)
	}
}

func TestToProtoMessageDescriptor_NillableFieldUsesProto3Optional(t *testing.T) {
	nickname := &gen.Field{
		Name:        "nickname",
		Type:        &field.TypeInfo{Type: field.TypeString},
		Nillable:    true,
		Annotations: map[string]any{FieldAnnotation: Field(2)},
	}
	node := &gen.Type{
		Name: "User",
		ID: &gen.Field{
			Name:        "id",
			Type:        &field.TypeInfo{Type: field.TypeInt64},
			UserDefined: true,
			Annotations: map[string]any{FieldAnnotation: Field(1)},
		},
		Fields:      []*gen.Field{nickname},
		Annotations: map[string]any{MessageAnnotation: Message()},
	}

	message, err := (&Adapter{}).toProtoMessageDescriptor(node)
	if err != nil {
		t.Fatalf("toProtoMessageDescriptor failed: %v", err)
	}
	protoField := message.Field[1]
	if !protoField.GetProto3Optional() {
		t.Fatal("nillable field is not marked proto3 optional")
	}
	if protoField.OneofIndex == nil || len(message.OneofDecl) != 1 || message.OneofDecl[0].GetName() != "_nickname" {
		t.Fatalf("invalid synthetic oneof for optional field: field=%v oneofs=%v", protoField, message.OneofDecl)
	}
}

func TestLoadAdapter_DedupesCrossPackageDependencies(t *testing.T) {
	schemaPath := "./testdata/schema/multipkg"
	g, err := entc.LoadGraph(schemaPath, &gen.Config{
		Target:  filepath.Join(t.TempDir(), "ent"),
		IDType:  &field.TypeInfo{Type: field.TypeInt64},
		Package: "github.com/go-sphere/entc-extensions/entproto/testdata/ent",
	})
	if err != nil {
		t.Fatalf("LoadGraph failed: %v", err)
	}

	a, err := LoadAdapter(g)
	if err != nil {
		t.Fatalf("LoadAdapter failed: %v", err)
	}

	fd, err := a.GetFileDescriptor("Group")
	if err != nil {
		t.Fatalf("GetFileDescriptor(Group) failed: %v", err)
	}

	deps := fd.GetDependencies()
	if len(deps) != 1 {
		t.Fatalf("dependency count=%d, want 1; deps=%v", len(deps), deps)
	}
	const want = "acme/user/v1/v1.proto"
	if deps[0].GetName() != want {
		t.Fatalf("dependency=%q, want %q", deps[0].GetName(), want)
	}
}

func TestLoadAdapter_MessageField(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	schemaPath := "./testdata/schema/customtype"
	g, err := entc.LoadGraph(schemaPath, &gen.Config{
		Target:  filepath.Join(t.TempDir(), "ent"),
		IDType:  &field.TypeInfo{Type: field.TypeInt64},
		Package: "github.com/go-sphere/entc-extensions/entproto/testdata/ent",
	})
	if err != nil {
		t.Fatalf("LoadGraph failed: %v", err)
	}

	a, err := LoadAdapter(g)
	if err != nil {
		t.Fatalf("LoadAdapter failed: %v", err)
	}

	fd, err := a.GetFileDescriptor("Admin")
	if err != nil {
		t.Fatalf("GetFileDescriptor(Admin) failed: %v", err)
	}

	const stubFile = "google/protobuf/timestamp.proto"
	deps := fd.GetDependencies()
	if len(deps) != 1 {
		t.Fatalf("dependency count=%d, want 1; deps=%v", len(deps), deps)
	}
	if got := deps[0].GetName(); got != stubFile {
		t.Fatalf("dependency=%q, want %q", got, stubFile)
	}

	msg := fd.FindMessage(fd.GetPackage() + ".Admin")
	if msg == nil {
		t.Fatalf("could not find Admin message in %s", fd.GetName())
	}
	tsField := msg.FindFieldByName("ts")
	if tsField == nil {
		t.Fatalf("Admin.ts field not found")
	}
	if gotType := tsField.AsFieldDescriptorProto().GetType(); gotType != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		t.Fatalf("ts field type=%v, want TYPE_MESSAGE", gotType)
	}
	if gotName := tsField.AsFieldDescriptorProto().GetTypeName(); gotName != ".google.protobuf.Timestamp" {
		t.Fatalf("ts field type_name=%q, want %q", gotName, ".google.protobuf.Timestamp")
	}

	if _, ext := a.GeneratedFileDescriptors()[stubFile]; ext {
		t.Fatalf("generated descriptors should not include the stub file %s", stubFile)
	}
	if _, all := a.AllFileDescriptors()[stubFile]; !all {
		t.Fatalf("AllFileDescriptors should still expose the stub for linking; missing %s", stubFile)
	}
}

func TestRegisterCustomType_FromMessage(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	RegisterCustomType(&timestamppb.Timestamp{})

	entry, ok := lookupCustomType("google.protobuf.Timestamp")
	if !ok {
		t.Fatalf("expected google.protobuf.Timestamp to be registered")
	}
	if entry.ProtoFile != "google/protobuf/timestamp.proto" {
		t.Fatalf("ProtoFile=%q, want google/protobuf/timestamp.proto", entry.ProtoFile)
	}
	if entry.ProtoPackage != "google.protobuf" || entry.MessageName != "Timestamp" {
		t.Fatalf("entry=%+v, want pkg=google.protobuf name=Timestamp", entry)
	}

	// Re-registering with the same descriptor is a no-op.
	RegisterCustomType(&timestamppb.Timestamp{})

	// Lookup also accepts a leading dot.
	if _, ok := lookupCustomType(".google.protobuf.Timestamp"); !ok {
		t.Fatalf("lookup with leading dot failed")
	}
}

func TestRegisterCustomType_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil message")
		}
	}()
	RegisterCustomType(nil)
}

func TestAddCustomTypeToStub_AggregatesTopLevelAndNestedMessages(t *testing.T) {
	files := map[string]*descriptorpb.FileDescriptorProto{}
	entries := []customTypeEntry{
		{ProtoFile: "shared/v1/types.proto", ProtoPackage: "shared.v1", MessageName: "User", MessagePath: "User"},
		{ProtoFile: "shared/v1/types.proto", ProtoPackage: "shared.v1", MessageName: "Address", MessagePath: "Address"},
		{ProtoFile: "shared/v1/types.proto", ProtoPackage: "shared.v1", MessageName: "Inner", MessagePath: "Outer.Inner"},
	}
	for _, entry := range entries {
		if err := addCustomTypeToStub(files, entry); err != nil {
			t.Fatalf("add custom type %+v: %v", entry, err)
		}
	}

	stub := files["shared/v1/types.proto"]
	if stub == nil {
		t.Fatal("missing aggregated stub")
	}
	if len(stub.MessageType) != 3 {
		t.Fatalf("top-level messages = %d, want 3", len(stub.MessageType))
	}
	outer := findMessage(stub.MessageType, "Outer")
	if outer == nil || findMessage(outer.NestedType, "Inner") == nil {
		t.Fatalf("nested message Outer.Inner missing from stub: %v", stub)
	}
}

func findMessage(messages []*descriptorpb.DescriptorProto, name string) *descriptorpb.DescriptorProto {
	for _, message := range messages {
		if message.GetName() == name {
			return message
		}
	}
	return nil
}

// TestLinkDependencyError_MessageFieldReference covers a schema that references
// a failed schema through a message-typed field (not an edge). The comment on
// linkDependencyError promises this case; before the fix only edges were
// checked, so the failure surfaced as an opaque descriptor-link error.
func TestLinkDependencyError_MessageFieldReference(t *testing.T) {
	b := &gen.Type{
		Name:        "B",
		ID:          &gen.Field{Name: "id", Type: &field.TypeInfo{Type: field.TypeInt64}},
		Fields:      []*gen.Field{},
		Edges:       []*gen.Edge{},
		Annotations: map[string]any{MessageAnnotation: Message()},
	}
	a := &gen.Type{
		Name: "A",
		ID:   &gen.Field{Name: "id", Type: &field.TypeInfo{Type: field.TypeInt64}},
		Fields: []*gen.Field{{
			Name: "bref",
			Type: &field.TypeInfo{Type: field.TypeInt},
			Annotations: map[string]any{
				FieldAnnotation: Field(3,
					Type(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
					TypeName("entpb.B"),
				),
			},
		}},
		Edges:       []*gen.Edge{},
		Annotations: map[string]any{MessageAnnotation: Message()},
	}
	graph := &gen.Graph{Nodes: []*gen.Type{a, b}}
	adapter := &Adapter{
		graph:      graph,
		nodeByName: map[string]*gen.Type{"A": a, "B": b},
		errors:     map[string]error{"B": errors.New("unsupported field type")},
	}

	err := adapter.linkDependencyError()
	if err == nil {
		t.Fatal("expected DanglingReferenceError for message-field reference")
	}
	var dangling *DanglingReferenceError
	if !errors.As(err, &dangling) {
		t.Fatalf("expected *DanglingReferenceError, got %T (%v)", err, err)
	}
	if dangling.Schema != "A" || dangling.Field != "bref" || dangling.RefSchema != "B" {
		t.Fatalf("dangling = %+v, want A.bref -> B", dangling)
	}
}

func testType(name string, msg any, fields []*gen.Field, edges []*gen.Edge) *gen.Type {
	return &gen.Type{
		Name:        name,
		ID:          &gen.Field{Name: "id", Type: &field.TypeInfo{Type: field.TypeInt64}},
		Fields:      fields,
		Edges:       edges,
		Annotations: map[string]any{MessageAnnotation: msg},
	}
}

func loadTestAdapter(t *testing.T, nodes ...*gen.Type) *Adapter {
	t.Helper()
	a, err := LoadAdapter(&gen.Graph{Nodes: nodes, Config: &gen.Config{Package: "example.com/app/ent"}})
	if err != nil {
		t.Fatalf("LoadAdapter failed: %v", err)
	}
	return a
}

// ent's pascal rewrites acronyms (Api -> API), so edge targets must be looked
// up by their schema name as-is.
func TestLoadAdapter_EdgeToAcronymNamedSchema(t *testing.T) {
	api := testType("Api", Message(), nil, nil)
	src := testType("Src", Message(), nil, []*gen.Edge{{
		Name: "api", Type: api, Unique: true,
		Annotations: map[string]any{FieldAnnotation: Field(2)},
	}})
	a := loadTestAdapter(t, api, src)
	md, err := a.GetMessageDescriptor("Src")
	if err != nil {
		t.Fatalf("GetMessageDescriptor(Src) failed: %v", err)
	}
	if got := md.FindFieldByName("api").GetMessageType().GetFullyQualifiedName(); got != "entpb.Api" {
		t.Fatalf("api edge type = %q, want entpb.Api", got)
	}
}

func TestLoadAdapter_SkippedEdgeToSkipGenSchema(t *testing.T) {
	hidden := testType("Hidden", SkipGen(), nil, nil)
	src := testType("Src", Message(), nil, []*gen.Edge{{
		Name: "hidden", Type: hidden,
		Annotations: map[string]any{SkipAnnotation: Skip()},
	}})
	a := loadTestAdapter(t, hidden, src)
	if _, err := a.GetMessageDescriptor("Src"); err != nil {
		t.Fatalf("GetMessageDescriptor(Src) failed: %v", err)
	}
	if _, err := a.GetMessageDescriptor("Hidden"); !errors.Is(err, ErrSchemaSkipped) {
		t.Fatalf("GetMessageDescriptor(Hidden) error = %v, want ErrSchemaSkipped", err)
	}
}

func TestLoadAdapter_UnskippedEdgeToSkipGenSchemaFails(t *testing.T) {
	hidden := testType("Hidden", SkipGen(), nil, nil)
	src := testType("Src", Message(), nil, []*gen.Edge{{
		Name: "hidden", Type: hidden,
		Annotations: map[string]any{FieldAnnotation: Field(2)},
	}})
	a := loadTestAdapter(t, hidden, src)
	_, err := a.GetMessageDescriptor("Src")
	if err == nil || !strings.Contains(err.Error(), "entproto.Skip()") {
		t.Fatalf("GetMessageDescriptor(Src) error = %v, want a hint to annotate the edge with entproto.Skip()", err)
	}
}

func TestLoadAdapter_EmptyPackageNameSharesDefaultPackage(t *testing.T) {
	u := testType("U", Message(PackageName("")), nil, nil)
	src := testType("Src", Message(), nil, []*gen.Edge{{
		Name: "u", Type: u, Unique: true,
		Annotations: map[string]any{FieldAnnotation: Field(2)},
	}})
	a := loadTestAdapter(t, u, src)
	md, err := a.GetMessageDescriptor("Src")
	if err != nil {
		t.Fatalf("GetMessageDescriptor(Src) failed: %v", err)
	}
	if got := md.FindFieldByName("u").GetMessageType().GetFullyQualifiedName(); got != "entpb.U" {
		t.Fatalf("u edge type = %q, want entpb.U", got)
	}
}

func TestLoadAdapter_InvalidFieldNumbersArePerSchemaErrors(t *testing.T) {
	tests := []struct {
		number int
		want   string
	}{
		{number: 0, want: "outside the protobuf range"},
		{number: -3, want: "outside the protobuf range"},
		{number: 19500, want: "reserved by protobuf"},
		{number: maxFieldNumber + 1, want: "outside the protobuf range"},
	}
	for _, tc := range tests {
		bad := testType("Bad", Message(), []*gen.Field{{
			Name: "f", Type: &field.TypeInfo{Type: field.TypeString},
			Annotations: map[string]any{FieldAnnotation: Field(tc.number)},
		}}, nil)
		good := testType("Good", Message(), nil, nil)
		a := loadTestAdapter(t, bad, good)
		if _, err := a.GetMessageDescriptor("Good"); err != nil {
			t.Fatalf("number %d: GetMessageDescriptor(Good) failed: %v", tc.number, err)
		}
		_, err := a.GetMessageDescriptor("Bad")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("number %d: error = %v, want it to contain %q", tc.number, err, tc.want)
		}
	}
}

func TestLoadAdapter_FieldNumberOverflowIsTyped(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("int cannot exceed int32 on this platform")
	}
	bad := testType("Bad", Message(), []*gen.Field{{
		Name: "f", Type: &field.TypeInfo{Type: field.TypeString},
		Annotations: map[string]any{FieldAnnotation: Field(math.MaxInt32 + 1)},
	}}, nil)
	_, err := loadTestAdapter(t, bad).GetMessageDescriptor("Bad")
	var overflow *FieldNumberOverflowError
	if !errors.As(err, &overflow) || overflow.Schema != "Bad" || overflow.Field != "f" {
		t.Fatalf("error = %v, want *FieldNumberOverflowError for Bad.f", err)
	}
}

func TestLoadAdapter_TypeOverrideRequiresTypeName(t *testing.T) {
	bad := testType("Bad", Message(), []*gen.Field{{
		Name: "f", Type: &field.TypeInfo{Type: field.TypeString},
		Annotations: map[string]any{FieldAnnotation: Field(2, Type(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE))},
	}}, nil)
	_, err := loadTestAdapter(t, bad).GetMessageDescriptor("Bad")
	if err == nil || !strings.Contains(err.Error(), "without entproto.TypeName") {
		t.Fatalf("error = %v, want missing TypeName error", err)
	}
}

func TestLoadAdapter_MalformedMessageAnnotationIsAnError(t *testing.T) {
	bad := testType("Bad", map[string]any{"Generate": "yes"}, nil, nil)
	_, err := loadTestAdapter(t, bad).GetMessageDescriptor("Bad")
	if err == nil || errors.Is(err, ErrSchemaSkipped) {
		t.Fatalf("error = %v, want a decode error rather than ErrSchemaSkipped", err)
	}
}

func TestLoadAdapter_UnsupportedFieldTypeNamesField(t *testing.T) {
	bad := testType("Bad", Message(), []*gen.Field{{
		Name: "blob", Type: &field.TypeInfo{Type: field.TypeJSON, Ident: "map[string]any"},
		Annotations: map[string]any{FieldAnnotation: Field(2)},
	}}, nil)
	_, err := loadTestAdapter(t, bad).GetMessageDescriptor("Bad")
	if err == nil || !strings.Contains(err.Error(), `"blob"`) {
		t.Fatalf("error = %v, want it to name the field", err)
	}
}

func TestLoadAdapter_EnumValueCollidingWithUnspecified(t *testing.T) {
	bad := testType("Bad", Message(), []*gen.Field{{
		Name:  "status",
		Type:  &field.TypeInfo{Type: field.TypeEnum},
		Enums: []gen.Enum{{Name: "Unspecified", Value: "unspecified"}, {Name: "On", Value: "on"}},
		Annotations: map[string]any{
			FieldAnnotation: Field(2),
			EnumAnnotation:  Enum(map[string]int32{"unspecified": 1, "on": 2}),
		},
	}}, nil)
	good := testType("Good", Message(), nil, nil)
	a := loadTestAdapter(t, bad, good)
	_, err := a.GetMessageDescriptor("Bad")
	if err == nil || !strings.Contains(err.Error(), "STATUS_UNSPECIFIED") {
		t.Fatalf("error = %v, want a STATUS_UNSPECIFIED collision", err)
	}
}

func TestEnumVerify_CountMismatchWrapsSentinel(t *testing.T) {
	fld := &gen.Field{
		Name:  "status",
		Type:  &field.TypeInfo{Type: field.TypeEnum},
		Enums: []gen.Enum{{Name: "On", Value: "on"}, {Name: "Off", Value: "off"}},
	}
	err := Enum(map[string]int32{"on": 1}).Verify(fld)
	if err == nil || !errors.Is(err, ErrEnumFieldsNotAnnotated) || !strings.Contains(err.Error(), `"status"`) {
		t.Fatalf("error = %v, want ErrEnumFieldsNotAnnotated naming the field", err)
	}
}
