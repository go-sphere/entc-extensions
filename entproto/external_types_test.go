package entproto

import (
	"errors"
	"strings"
	"testing"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/typepb"
)

func annotatedField(name string, typ field.Type, ident string, annotations ...any) *gen.Field {
	f := &gen.Field{Name: name, Type: &field.TypeInfo{Type: typ, Ident: ident}, Annotations: map[string]any{}}
	for _, a := range annotations {
		switch a := a.(type) {
		case pbfield:
			f.Annotations[FieldAnnotation] = a
		case *enum:
			f.Annotations[EnumAnnotation] = a
		}
	}
	return f
}

func TestLoadAdapter_EnumField(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	s := testType("S", Message(), []*gen.Field{
		annotatedField("null_marker", field.TypeInt32, "structpb.NullValue",
			EnumField(2, structpb.NullValue_NULL_VALUE)),
		annotatedField("kind", field.TypeInt32, "typepb.Field_Kind",
			EnumField(3, typepb.Field_TYPE_STRING)),
	}, nil)
	s.Fields[1].Nillable = true
	a := loadTestAdapter(t, s)

	md, err := a.GetMessageDescriptor("S")
	if err != nil {
		t.Fatalf("GetMessageDescriptor(S) failed: %v", err)
	}
	if got := md.FindFieldByName("null_marker").GetEnumType().GetFullyQualifiedName(); got != "google.protobuf.NullValue" {
		t.Fatalf("null_marker enum = %q, want google.protobuf.NullValue", got)
	}
	kind := md.FindFieldByName("kind")
	if got := kind.GetEnumType().GetFullyQualifiedName(); got != "google.protobuf.Field.Kind" {
		t.Fatalf("kind enum = %q, want google.protobuf.Field.Kind", got)
	}
	if !kind.AsFieldDescriptorProto().GetProto3Optional() {
		t.Fatal("nillable external enum field is not proto3 optional")
	}

	var deps []string
	for _, dep := range md.GetFile().GetDependencies() {
		deps = append(deps, dep.GetName())
	}
	if strings.Join(deps, ",") != "google/protobuf/struct.proto,google/protobuf/type.proto" {
		t.Fatalf("dependencies = %v, want struct.proto and type.proto", deps)
	}
	for _, stub := range []string{"google/protobuf/struct.proto", "google/protobuf/type.proto"} {
		if _, generated := a.GeneratedFileDescriptors()[stub]; generated {
			t.Fatalf("stub %s must not be generated", stub)
		}
	}

	fm, err := a.FieldMap("S")
	if err != nil {
		t.Fatalf("FieldMap(S) failed: %v", err)
	}
	m := fm["null_marker"]
	if m == nil {
		t.Fatal("null_marker missing from field map")
	}
	if !m.IsEnumField || m.IsEntEnum() {
		t.Fatalf("null_marker: IsEnumField=%v IsEntEnum=%v, want true/false", m.IsEnumField, m.IsEntEnum())
	}
}

func TestRegisterCustomEnum(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	RegisterCustomEnum(structpb.NullValue_NULL_VALUE)
	s := testType("S", Message(), []*gen.Field{
		annotatedField("null_marker", field.TypeInt32, "", Field(2,
			Type(descriptorpb.FieldDescriptorProto_TYPE_ENUM),
			TypeName("google.protobuf.NullValue"),
		)),
	}, nil)
	if _, err := loadTestAdapter(t, s).GetMessageDescriptor("S"); err != nil {
		t.Fatalf("GetMessageDescriptor(S) failed: %v", err)
	}
}

func TestEnumField_RejectsClosedEnum(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "closed (proto2) enum") {
			t.Fatalf("recover() = %v, want closed enum panic", r)
		}
	}()
	EnumField(2, descriptorpb.FieldDescriptorProto_TYPE_INT32)
}

func TestMessageField_RejectsFileWithoutPackage(t *testing.T) {
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:        toPtr("bare.proto"),
		Syntax:      toPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{Name: toPtr("Bare")}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	msg := dynamicpb.NewMessage(file.Messages().Get(0))
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "has no package statement") {
			t.Fatalf("recover() = %v, want missing package panic", r)
		}
	}()
	MessageField(2, msg)
}

func TestLoadAdapter_CustomTypeConflictIsSchemaError(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	conflicting := MessageField(2, &timestamppb.Timestamp{}).(pbfield)
	conflicting.ProtoFile = "elsewhere/timestamp.proto"
	bad := testType("Bad", Message(), []*gen.Field{
		annotatedField("a", field.TypeJSON, "*timestamppb.Timestamp", MessageField(2, &timestamppb.Timestamp{})),
		annotatedField("b", field.TypeJSON, "*timestamppb.Timestamp", func() pbfield { c := conflicting; c.Number = 3; return c }()),
	}, nil)
	good := testType("Good", Message(), nil, nil)
	a := loadTestAdapter(t, bad, good)
	if _, err := a.GetMessageDescriptor("Good"); err != nil {
		t.Fatalf("GetMessageDescriptor(Good) failed: %v", err)
	}
	_, err := a.GetMessageDescriptor("Bad")
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("error = %v, want custom type conflict", err)
	}
}

func TestLoadAdapter_TypeNameMustMatchFullName(t *testing.T) {
	timestamp := testType("Timestamp", Message(), nil, nil)
	ref := testType("Ref", Message(), []*gen.Field{
		annotatedField("ts", field.TypeJSON, "*timestamppb.Timestamp", Field(2,
			Type(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
			TypeName("google.protobuf.Timestamp"),
		)),
	}, nil)
	a := loadTestAdapter(t, timestamp, ref)
	_, err := a.GetMessageDescriptor("Ref")
	if err == nil || !strings.Contains(err.Error(), `unknown protobuf type "google.protobuf.Timestamp"`) {
		t.Fatalf("error = %v, want unknown type instead of matching schema Timestamp", err)
	}
}

func TestLoadAdapter_TypeNameReferencesSchemaEnumAcrossPackages(t *testing.T) {
	owner := testType("Owner", Message(PackageName("acme.owner.v1")), []*gen.Field{
		annotatedField("status", field.TypeEnum, "", Field(2), Enum(map[string]int32{"on": 1})),
	}, nil)
	owner.Fields[0].Enums = []gen.Enum{{Name: "On", Value: "on"}}
	ref := testType("Ref", Message(), []*gen.Field{
		annotatedField("status", field.TypeInt32, "", Field(2,
			Type(descriptorpb.FieldDescriptorProto_TYPE_ENUM),
			TypeName("acme.owner.v1.Owner.Status"),
		)),
	}, nil)
	md, err := loadTestAdapter(t, owner, ref).GetMessageDescriptor("Ref")
	if err != nil {
		t.Fatalf("GetMessageDescriptor(Ref) failed: %v", err)
	}
	deps := md.GetFile().GetDependencies()
	if len(deps) != 1 || deps[0].GetName() != "acme/owner/v1/v1.proto" {
		t.Fatalf("dependencies = %v, want acme/owner/v1/v1.proto", deps)
	}
}

func TestLoadAdapter_EnumTypeNameRegisteredAsMessage(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	RegisterCustomType(&timestamppb.Timestamp{})
	bad := testType("Bad", Message(), []*gen.Field{
		annotatedField("ts", field.TypeInt32, "", Field(2,
			Type(descriptorpb.FieldDescriptorProto_TYPE_ENUM),
			TypeName("google.protobuf.Timestamp"),
		)),
	}, nil)
	_, err := loadTestAdapter(t, bad).GetMessageDescriptor("Bad")
	if err == nil || !strings.Contains(err.Error(), "registered as a message") {
		t.Fatalf("error = %v, want kind mismatch", err)
	}
}

func TestLoadAdapter_JSONSliceOverrideIsRepeated(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	s := testType("S", Message(), []*gen.Field{
		annotatedField("times", field.TypeJSON, "[]*timestamppb.Timestamp", MessageField(2, &timestamppb.Timestamp{})),
		annotatedField("ids", field.TypeJSON, "[]int64", Field(3, Type(descriptorpb.FieldDescriptorProto_TYPE_SINT64))),
		annotatedField("raw", field.TypeJSON, "[]byte", Field(4, Type(descriptorpb.FieldDescriptorProto_TYPE_BYTES))),
		annotatedField("one", field.TypeJSON, "*timestamppb.Timestamp", MessageField(5, &timestamppb.Timestamp{})),
	}, nil)
	md, err := loadTestAdapter(t, s).GetMessageDescriptor("S")
	if err != nil {
		t.Fatalf("GetMessageDescriptor(S) failed: %v", err)
	}
	for name, want := range map[string]bool{"times": true, "ids": true, "raw": false, "one": false} {
		if got := md.FindFieldByName(name).IsRepeated(); got != want {
			t.Errorf("%s repeated = %v, want %v", name, got, want)
		}
	}
}

func TestLoadAdapter_OmitFieldPrefixCollisionAcrossFields(t *testing.T) {
	enumField := func(name string, num int) *gen.Field {
		f := annotatedField(name, field.TypeEnum, "", Field(num), Enum(map[string]int32{"active": 1}, OmitFieldPrefix()))
		f.Enums = []gen.Enum{{Name: "Active", Value: "active"}}
		return f
	}
	bad := testType("Bad", Message(), []*gen.Field{enumField("state", 2), enumField("phase", 3)}, nil)
	good := testType("Good", Message(), nil, nil)
	a := loadTestAdapter(t, bad, good)
	_, err := a.GetMessageDescriptor("Bad")
	if err == nil || !strings.Contains(err.Error(), `collides with a value of field "state"`) {
		t.Fatalf("error = %v, want a cross-field enum value collision", err)
	}
	if errors.Is(err, ErrSchemaSkipped) {
		t.Fatal("collision must not be reported as a skipped schema")
	}
}

func TestLoadAdapter_EnumOverrideSkipsNestedEnumAndIsEntEnumIsFalse(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	owner := testType("Owner", Message(PackageName("acme.owner.v1")), []*gen.Field{
		func() *gen.Field {
			f := annotatedField("status", field.TypeEnum, "", Field(2), Enum(map[string]int32{"on": 1}))
			f.Enums = []gen.Enum{{Name: "On", Value: "on"}}
			return f
		}(),
	}, nil)

	enumWithOpts := func(name string, ann pbfield) *gen.Field {
		f := annotatedField(name, field.TypeEnum, "", ann)
		f.Enums = []gen.Enum{{Name: "A", Value: "a"}, {Name: "B", Value: "b"}}
		return f
	}
	s := testType("S", Message(), []*gen.Field{
		enumWithOpts("ext", EnumField(2, structpb.NullValue_NULL_VALUE).(pbfield)),
		enumWithOpts("str", Field(3, Type(descriptorpb.FieldDescriptorProto_TYPE_STRING)).(pbfield)),
		enumWithOpts("cross", Field(4, Type(descriptorpb.FieldDescriptorProto_TYPE_ENUM), TypeName("acme.owner.v1.Owner.Status")).(pbfield)),
	}, nil)

	a := loadTestAdapter(t, owner, s)
	md, err := a.GetMessageDescriptor("S")
	if err != nil {
		t.Fatalf("GetMessageDescriptor(S) failed: %v", err)
	}
	if got := len(md.GetNestedEnumTypes()); got != 0 {
		t.Fatalf("nested enum count = %d, want 0", got)
	}
	fm, err := a.FieldMap("S")
	if err != nil {
		t.Fatalf("FieldMap(S) failed: %v", err)
	}
	for _, name := range []string{"ext", "str", "cross"} {
		m := fm[name]
		if m == nil {
			t.Fatalf("missing field %q in FieldMap", name)
		}
		if m.IsEntEnum() {
			t.Fatalf("field %q: IsEntEnum() = true, want false", name)
		}
	}
}

func TestLoadAdapter_RejectsExternalProtoFileCollidingWithGeneratedProto(t *testing.T) {
	resetCustomTypeRegistry()
	t.Cleanup(resetCustomTypeRegistry)

	colliding := MessageField(2, &timestamppb.Timestamp{}).(pbfield)
	colliding.ProtoFile = "acme/target/v1/v1.proto"

	target := testType("Target", Message(PackageName("acme.target.v1")), nil, nil)
	bad := testType("Bad", Message(), []*gen.Field{
		annotatedField("ts", field.TypeJSON, "*timestamppb.Timestamp", colliding),
	}, nil)
	good := testType("Good", Message(PackageName("acme.good.v1")), nil, nil)

	a := loadTestAdapter(t, bad, target, good)
	_, err := a.GetMessageDescriptor("Bad")
	if err == nil || !strings.Contains(err.Error(), "collides with generated proto file") {
		t.Fatalf("GetMessageDescriptor(Bad) error = %v, want collision error", err)
	}
	if _, err := a.GetMessageDescriptor("Target"); err != nil {
		t.Fatalf("GetMessageDescriptor(Target) failed: %v", err)
	}
	if _, ok := a.GeneratedFileDescriptors()["acme/target/v1/v1.proto"]; !ok {
		t.Fatal("Target's generated file descriptor missing after collision rejection on Bad")
	}

	// Also reject uncleaned relative paths colliding with own package file.
	selfCollide := MessageField(2, &timestamppb.Timestamp{}).(pbfield)
	selfCollide.ProtoFile = "./entpb/entpb.proto"
	badSelf := testType("BadSelf", Message(), []*gen.Field{
		annotatedField("ts", field.TypeJSON, "*timestamppb.Timestamp", selfCollide),
	}, nil)
	a2 := loadTestAdapter(t, badSelf, good)
	if _, err := a2.GetMessageDescriptor("BadSelf"); err == nil || !strings.Contains(err.Error(), "collides with generated proto file") {
		t.Fatalf("GetMessageDescriptor(BadSelf) error = %v, want collision error", err)
	}
}
