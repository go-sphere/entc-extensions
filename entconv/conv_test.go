package entconv

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"entgo.io/ent/entc/gen"
	"github.com/go-sphere/entc-extensions/entconv/internal/generator"
	"github.com/go-sphere/entc-extensions/entproto"
	"github.com/jhump/protoreflect/desc/builder"
)

const (
	testEntPackagePath   = "github.com/acme/project/internal/pkg/database/ent"
	testProtoPackagePath = "github.com/go-sphere/entc-extensions/entconv/testdata/fixtures/pb"
)

func TestGenerateConverter_SequentialCallsUseCurrentProtoAlias(t *testing.T) {
	first, err := GenerateConverter(testOptions(t, "pbA"))
	if err != nil {
		t.Fatalf("first GenerateConverter failed: %v", err)
	}
	firstCode := string(first)
	if firstCode == "" {
		t.Fatal("first GenerateConverter produced empty output")
	}
	if !strings.Contains(firstCode, `pbA "`+testProtoPackagePath+`"`) {
		t.Fatalf("first output does not contain expected proto import alias; output:\n%s", firstCode)
	}

	second, err := GenerateConverter(testOptions(t, "pbB"))
	if err != nil {
		t.Fatalf("second GenerateConverter failed: %v", err)
	}
	secondCode := string(second)
	if secondCode == "" {
		t.Fatal("second GenerateConverter produced empty output")
	}

	if !strings.Contains(secondCode, `pbB "`+testProtoPackagePath+`"`) {
		t.Fatalf("second output does not contain expected proto import alias; output:\n%s", secondCode)
	}
	if strings.Contains(secondCode, "pbA.") {
		t.Fatalf("second output leaked first alias pbA; output:\n%s", secondCode)
	}
	if !strings.Contains(secondCode, "func ToProtoUser(") {
		t.Fatalf("second output missing ToProtoUser; output:\n%s", secondCode)
	}
	if !strings.Contains(secondCode, "func ToEntUser(") {
		t.Fatalf("second output missing ToEntUser; output:\n%s", secondCode)
	}
	if !strings.Contains(secondCode, "*pbB.User") {
		t.Fatalf("second output does not use current alias in signatures; output:\n%s", secondCode)
	}
	if !strings.Contains(secondCode, "e.Nickname = new(*v.Nickname)") {
		t.Fatalf("second output does not preserve nillable nickname; output:\n%s", secondCode)
	}
}

func TestGenerateConverterWithOptions_SequentialCallsUseCurrentProtoAlias(t *testing.T) {
	first, err := GenerateConverterWithOptions(testOptionFuncs(t, "pbA")...)
	if err != nil {
		t.Fatalf("first GenerateConverterWithOptions failed: %v", err)
	}
	firstCode := string(first)
	if firstCode == "" {
		t.Fatal("first GenerateConverterWithOptions produced empty output")
	}
	if !strings.Contains(firstCode, `pbA "`+testProtoPackagePath+`"`) {
		t.Fatalf("first output does not contain expected proto import alias; output:\n%s", firstCode)
	}

	second, err := GenerateConverterWithOptions(testOptionFuncs(t, "pbB")...)
	if err != nil {
		t.Fatalf("second GenerateConverterWithOptions failed: %v", err)
	}
	secondCode := string(second)
	if secondCode == "" {
		t.Fatal("second GenerateConverterWithOptions produced empty output")
	}
	if !strings.Contains(secondCode, `pbB "`+testProtoPackagePath+`"`) {
		t.Fatalf("second output does not contain expected proto import alias; output:\n%s", secondCode)
	}
	if strings.Contains(secondCode, "pbA.") {
		t.Fatalf("second output leaked first alias pbA; output:\n%s", secondCode)
	}
}

func TestGenerateConverterFileWithOptions_GeneratesFile(t *testing.T) {
	outDir := t.TempDir()
	opts := append(testOptionFuncs(t, "pbX"), WithOutDir(outDir))
	if err := GenerateConverterFileWithOptions(opts...); err != nil {
		t.Fatalf("GenerateConverterFileWithOptions failed: %v", err)
	}

	generated := filepath.Join(outDir, "user.go")
	content, err := os.ReadFile(generated)
	if err != nil {
		t.Fatalf("reading generated file %q failed: %v", generated, err)
	}
	code := string(content)
	if !strings.Contains(code, "func ToProtoUser(") {
		t.Fatalf("generated file missing ToProtoUser; output:\n%s", code)
	}
	if !strings.Contains(code, `pbX "`+testProtoPackagePath+`"`) {
		t.Fatalf("generated file missing proto alias import; output:\n%s", code)
	}
}

func TestDefaultOptions_UsesScaffoldDefaults(t *testing.T) {
	opts := DefaultOptions()
	if opts.IDType != "int64" {
		t.Fatalf("IDType default = %q, want int64", opts.IDType)
	}
	if opts.SchemaPath != "./internal/pkg/database/schema" {
		t.Fatalf("SchemaPath default = %q", opts.SchemaPath)
	}
	if opts.ConvPackage != "entmap" {
		t.Fatalf("ConvPackage default = %q, want entmap", opts.ConvPackage)
	}
	if opts.ProtoFile != "./api/entpb/entpb.pb.go" {
		t.Fatalf("ProtoFile default = %q", opts.ProtoFile)
	}
	if opts.OutDir != "./internal/pkg/render/entmap" {
		t.Fatalf("OutDir default = %q", opts.OutDir)
	}
	if opts.ProtoAlias != "entpb" {
		t.Fatalf("ProtoAlias default = %q, want entpb", opts.ProtoAlias)
	}
	if opts.MissingProtoPolicy != MissingProtoPolicyStrict {
		t.Fatalf("MissingProtoPolicy default = %q, want %q", opts.MissingProtoPolicy, MissingProtoPolicyStrict)
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	wantEntPath := path.Join(info.Main.Path, "/internal/pkg/database/ent")
	wantProtoPath := path.Join(info.Main.Path, "/api/entpb")
	if opts.EntPackagePath != wantEntPath {
		t.Fatalf("EntPackagePath default = %q, want %q", opts.EntPackagePath, wantEntPath)
	}
	if opts.ProtoPackagePath != wantProtoPath {
		t.Fatalf("ProtoPackagePath default = %q, want %q", opts.ProtoPackagePath, wantProtoPath)
	}
}

func TestGenerateConverter_StrictMissingProtoPolicyFails(t *testing.T) {
	opts := testOptions(t, "entpb")
	opts.ProtoFile = writeProtoFixture(t, "missingpb", "NotUser", "")
	opts.MissingProtoPolicy = MissingProtoPolicyStrict

	_, err := GenerateConverter(opts)
	if err == nil {
		t.Fatal("expected error when proto message is missing")
	}
	var missing *MissingProtoMessagesError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingProtoMessagesError, got %T (%v)", err, err)
	}
	if len(missing.Missing) != 2 || missing.Missing[0] != "Post" || missing.Missing[1] != "User" {
		t.Fatalf("missing messages = %v, want [Post User]", missing.Missing)
	}
}

func TestGenerateConverter_WarnMissingProtoPolicyContinues(t *testing.T) {
	opts := testOptions(t, "warnpb")
	opts.ProtoFile = writeProtoFixture(t, "warnpb", "User", "Id int64\nName string\nNickname *string")
	opts.MissingProtoPolicy = MissingProtoPolicyWarn

	var warned error
	opts.WarningHandler = func(err error) {
		warned = err
	}

	code, err := GenerateConverter(opts)
	if err != nil {
		t.Fatalf("GenerateConverter failed with warn policy: %v", err)
	}
	if len(code) == 0 {
		t.Fatal("expected non-empty generated code")
	}
	if warned == nil {
		t.Fatal("expected warning callback to be called")
	}
	var missing *MissingProtoMessagesError
	if !errors.As(warned, &missing) {
		t.Fatalf("warning type = %T, want *MissingProtoMessagesError", warned)
	}
	if len(missing.Missing) != 1 || missing.Missing[0] != "Post" {
		t.Fatalf("missing messages = %v, want [Post]", missing.Missing)
	}
}

func TestGenerateConverter_RejectsIncompleteProtoMessage(t *testing.T) {
	opts := testOptions(t, "brokenpb")
	opts.ProtoFile = writeProtoFixture(t, "brokenpb", "User", "Id int64")
	opts.MissingProtoPolicy = MissingProtoPolicyWarn

	_, err := GenerateConverter(opts)
	var contract *ProtoFieldContractError
	if !errors.As(err, &contract) {
		t.Fatalf("expected ProtoFieldContractError, got %T (%v)", err, err)
	}
	if contract.Message != "User" || contract.Field != "Name" {
		t.Fatalf("contract error = %+v, want User.Name", contract)
	}
}

func TestGenerateConverter_RejectsMismatchedProtoFieldType(t *testing.T) {
	opts := testOptions(t, "brokenpb")
	opts.ProtoFile = writeProtoFixture(t, "brokenpb", "User", "Id string\nName string\nNickname *string")
	opts.MissingProtoPolicy = MissingProtoPolicyWarn

	_, err := GenerateConverter(opts)
	var contract *ProtoFieldContractError
	if !errors.As(err, &contract) {
		t.Fatalf("expected ProtoFieldContractError, got %T (%v)", err, err)
	}
	if contract.Field != "Id" || contract.Expected != "int64" || contract.Actual != "string" {
		t.Fatalf("contract error = %+v", contract)
	}
}

func TestValidateOptions_RejectsInvalidIDType(t *testing.T) {
	opts := testOptions(t, "entpb")
	opts.IDType = "uint32"

	err := validateOptions(opts)
	var invalid *InvalidIDTypeError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidIDTypeError, got %T (%v)", err, err)
	}
}

func TestProtoDescriptorGoName_NestedMessage(t *testing.T) {
	cases := []struct {
		full, pkg, want string
	}{
		{"google.protobuf.FieldOptions.FeatureSupport", "google.protobuf", "FieldOptions_FeatureSupport"},
		{"acme.v1.user_profile", "acme.v1", "UserProfile"},
		{"acme.v1.outer_msg.InnerKind", "acme.v1", "OuterMsg_InnerKind"},
		{"acme.v1.outer_msg.inner_kind", "acme.v1", "OuterMsgInnerKind"},
		{".acme.v1.my_item", "acme.v1", "MyItem"},
	}
	for _, tc := range cases {
		if got := protoDescriptorGoName(tc.full, tc.pkg); got != tc.want {
			t.Fatalf("protoDescriptorGoName(%q, %q) = %q, want %q", tc.full, tc.pkg, got, tc.want)
		}
	}
}

func TestExpectedProtoGoType_RepeatedBytesAndWireTypes(t *testing.T) {
	mb := builder.NewMessage("Wire").
		AddField(builder.NewField("blobs", builder.FieldTypeBytes()).SetNumber(2).SetRepeated()).
		AddField(builder.NewField("opt_blob", builder.FieldTypeBytes()).SetNumber(3).SetProto3Optional(true)).
		AddField(builder.NewField("s32", builder.FieldTypeSInt32()).SetNumber(4)).
		AddField(builder.NewField("s64", builder.FieldTypeSInt64()).SetNumber(5)).
		AddField(builder.NewField("f32", builder.FieldTypeFixed32()).SetNumber(6)).
		AddField(builder.NewField("f64", builder.FieldTypeFixed64()).SetNumber(7)).
		AddField(builder.NewField("sf32", builder.FieldTypeSFixed32()).SetNumber(8)).
		AddField(builder.NewField("sf64", builder.FieldTypeSFixed64()).SetNumber(9))
	file, err := builder.NewFile("wire.proto").SetProto3(true).AddMessage(mb).Build()
	if err != nil {
		t.Fatalf("build descriptor: %v", err)
	}
	msg := file.FindMessage("Wire")
	want := map[string]string{
		"blobs":    "[][]byte",
		"opt_blob": "[]byte",
		"s32":      "int32",
		"s64":      "int64",
		"f32":      "uint32",
		"f64":      "uint64",
		"sf32":     "int32",
		"sf64":     "int64",
	}
	for name, expected := range want {
		got := expectedProtoGoType(&entproto.FieldMappingDescriptor{PbFieldDescriptor: msg.FindFieldByName(name)})
		if got != expected {
			t.Errorf("%s: expectedProtoGoType = %q, want %q", name, got, expected)
		}
	}
}

func TestParseProtoFile_TopLevelTypesAndConstsOnly(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.pb.go")
	src := `package pb

type User struct {
	Id int64
}

const (
	User_STATUS_OK = 1
)

var File_test_proto = "ignored"

func helper() {
	type localOnly struct {
		X int
	}
	const localConst = 2
	var _ = localOnly{X: localConst}
}
`
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := parseProtoFile(file)
	if err != nil {
		t.Fatalf("parseProtoFile: %v", err)
	}
	if _, ok := info.Messages["User"]; !ok {
		t.Fatal("missing top-level message User")
	}
	if _, ok := info.Messages["localOnly"]; ok {
		t.Fatal("function-local struct localOnly must not be collected")
	}
	if _, ok := info.Consts["User_STATUS_OK"]; !ok {
		t.Fatal("missing top-level const User_STATUS_OK")
	}
	if _, ok := info.Consts["localConst"]; ok {
		t.Fatal("function-local const localConst must not be collected")
	}
	if _, ok := info.Consts["File_test_proto"]; ok {
		t.Fatal("top-level var File_test_proto must not be collected as const")
	}
}

func TestMatchTypes_SkipsSkipGenSchemas(t *testing.T) {
	g := &gen.Graph{
		Nodes: []*gen.Type{
			{Name: "User", Annotations: map[string]any{entproto.MessageAnnotation: entproto.Message()}},
			{Name: "SkipDirect", Annotations: map[string]any{entproto.MessageAnnotation: entproto.SkipGen()}},
			{Name: "SkipMap", Annotations: map[string]any{entproto.MessageAnnotation: map[string]any{"Generate": false}}},
		},
	}
	protoMsgs := map[string]*generator.ProtoMessage{
		"User": {Name: "User"},
	}
	matched, missing := matchTypes(g, protoMsgs)
	if missing != nil {
		t.Fatalf("expected SkipGen schemas to be skipped, got missing=%v", missing)
	}
	if len(matched) != 1 || matched[0].MessageName != "User" {
		t.Fatalf("matched = %+v, want only User", matched)
	}
}

func TestGenerateConverter_WithAutoFill(t *testing.T) {
	fixtureDir := filepath.Join(moduleRoot(t), "testdata", "fixtures", "autofillschema")
	pbFile := writeProtoFixture(t, "entpb", "Item", "Id int64\nTitle string")

	baseOpts := func(autoFill bool) *Options {
		opts := NewOptions(
			WithSchemaPath(fixtureDir),
			WithEntPackagePath(testEntPackagePath),
			WithProtoFile(pbFile),
			WithProtoPackagePath(testProtoPackagePath),
			WithProtoAlias("entpb"),
			WithOutDir(t.TempDir()),
		)
		opts.AutoFill = autoFill
		return opts
	}

	// Without AutoFill, unannotated Item fails strict missing-proto / contract check
	// (no message descriptor for Item).
	if _, err := GenerateConverter(baseOpts(false)); err == nil {
		t.Fatal("expected GenerateConverter without AutoFill to fail on unannotated schema")
	}

	// With AutoFill (via WithAutoFill()), Item gets auto-annotated and Hidden (SkipGen) is skipped.
	opts := NewOptions(
		WithSchemaPath(fixtureDir),
		WithEntPackagePath(testEntPackagePath),
		WithProtoFile(pbFile),
		WithProtoPackagePath(testProtoPackagePath),
		WithProtoAlias("entpb"),
		WithOutDir(t.TempDir()),
		WithAutoFill(),
	)
	code, err := GenerateConverter(opts)
	if err != nil {
		t.Fatalf("GenerateConverter with WithAutoFill failed: %v", err)
	}
	src := string(code)
	if !strings.Contains(src, "func ToProtoItem(") || !strings.Contains(src, "func ToEntItem(") {
		t.Fatalf("expected ToProtoItem/ToEntItem in generated output:\n%s", src)
	}
	if strings.Contains(src, "Hidden") {
		t.Fatalf("SkipGen schema Hidden should not appear in generated output:\n%s", src)
	}
}

func testOptions(t *testing.T, alias string) *Options {
	t.Helper()

	fixtureRoot := filepath.Join(moduleRoot(t), "testdata", "fixtures")
	return &Options{
		SchemaPath:       filepath.Join(fixtureRoot, "schema"),
		EntPackagePath:   testEntPackagePath,
		IDType:           "int64",
		ProtoFile:        filepath.Join(fixtureRoot, "pb", "fixture.pb.go"),
		ConvPackage:      "entmap",
		ProtoPackagePath: testProtoPackagePath,
		ProtoAlias:       alias,
		OutDir:           filepath.Join(fixtureRoot, "out"),
	}
}

func testOptionFuncs(t *testing.T, alias string) []Option {
	t.Helper()

	fixtureRoot := filepath.Join(moduleRoot(t), "testdata", "fixtures")
	return []Option{
		WithSchemaPath(filepath.Join(fixtureRoot, "schema")),
		WithEntPackagePath(testEntPackagePath),
		WithIDType("int64"),
		WithProtoFile(filepath.Join(fixtureRoot, "pb", "fixture.pb.go")),
		WithConvPackage("entmap"),
		WithProtoPackagePath(testProtoPackagePath),
		WithProtoAlias(alias),
		WithOutDir(filepath.Join(fixtureRoot, "out")),
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test file path")
	}
	return filepath.Dir(file)
}

func writeProtoFixture(t *testing.T, pkg, msg, fields string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "fixture.pb.go")
	content := fmt.Sprintf("package %s\ntype %s struct {\n%s\n}\n", pkg, msg, fields)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture pb.go: %v", err)
	}
	return file
}
