package entconv

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
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
