package entproto

import (
	"path/filepath"
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

	_, err := a.toProtoMessageDescriptor(node)
	if err != nil {
		t.Fatalf("toProtoMessageDescriptor returned error: %v", err)
	}
	if got := id.Annotations["ExistingKey"]; got != "keep-me" {
		t.Fatalf("existing ID annotation lost, got=%v", got)
	}
	if _, ok := id.Annotations[FieldAnnotation]; !ok {
		t.Fatalf("expected %s annotation to be added", FieldAnnotation)
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
