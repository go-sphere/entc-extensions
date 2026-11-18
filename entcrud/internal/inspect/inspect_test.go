package inspect

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-sphere/entc-extensions/entcrud/internal/strcase"
)

type testStruct struct {
	Name     string
	Age      int
	Email    *string
	Active   bool
	Score    float64
	Tags     []string
	Metadata map[string]string
	Flags    [2]bool
	Nested   struct {
		Code int
	}
}

func TestIndirectValue(t *testing.T) {
	original := "test"
	ptr := &original
	ptrPtr := &ptr

	if IndirectValue(reflect.ValueOf(ptrPtr)).String() != "test" {
		t.Error("IndirectValue failed to dereference multiple pointers")
	}
}

func TestTypeName(t *testing.T) {
	type MyStruct struct{}
	if TypeName(MyStruct{}) != "MyStruct" {
		t.Errorf("Expected MyStruct, got %s", TypeName(MyStruct{}))
	}
	if TypeName(&MyStruct{}) != "MyStruct" {
		t.Errorf("Expected MyStruct for pointer, got %s", TypeName(&MyStruct{}))
	}
}

func TestExtractPublicFields(t *testing.T) {
	keys, fields := ExtractPublicFields(testStruct{}, nil)
	if len(keys) != 9 {
		t.Errorf("Expected 9 fields, got %d", len(keys))
	}
	if _, ok := fields["Name"]; !ok {
		t.Error("Expected Name field")
	}
	if _, ok := fields["Email"]; !ok {
		t.Error("Expected Email field")
	}
}

func TestExtractPublicFields_TagsAndFallback(t *testing.T) {
	type taggedStruct struct {
		IpV4        int32  `protobuf:"varint,1,opt,name=ip_v4,json=ipV4,proto3" json:"ipV4,omitempty"`
		HTTPURL     string `json:"http_url,omitempty"`
		S3Key       string `json:"s3_key,omitempty"`
		SkipJSON    string `json:"-"`
		EmptyJSON   string `json:",omitempty"`
		SkipPB      string `protobuf:"-"`
		Ipv4Primary string `json:"ipv4"`
		IPV4Shadow  string `json:"shadow_ipv4"`
	}

	keys, fields := ExtractPublicFields(taggedStruct{}, strcase.ToSnake)
	wantKeys := []string{
		"ip_v4",
		"http_url",
		"s3_key",
		"skip_json",
		"empty_json",
		"skip_pb",
		"ipv4",
		"shadow_ipv4",
	}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("keys = %v, want %v", keys, wantKeys)
	}

	// Both canonical keys and strcase fallback keys resolve when unambiguous.
	if f, ok := fields["ip_v4"]; !ok || f.Name != "IpV4" {
		t.Fatalf("fields[ip_v4] = (%v, %v), want IpV4", f.Name, ok)
	}
	if f, ok := fields["http_url"]; !ok || f.Name != "HTTPURL" {
		t.Fatalf("fields[http_url] = (%v, %v), want HTTPURL", f.Name, ok)
	}
	if f, ok := fields["httpurl"]; !ok || f.Name != "HTTPURL" {
		t.Fatalf("fields[httpurl] = (%v, %v), want HTTPURL", f.Name, ok)
	}
	if f, ok := fields["s3_key"]; !ok || f.Name != "S3Key" {
		t.Fatalf("fields[s3_key] = (%v, %v), want S3Key", f.Name, ok)
	}
	if f, ok := fields["s3key"]; !ok || f.Name != "S3Key" {
		t.Fatalf("fields[s3key] = (%v, %v), want S3Key", f.Name, ok)
	}
	// Primary key "ipv4" from Ipv4Primary must not be overwritten by IPV4Shadow's alt key.
	if f, ok := fields["ipv4"]; !ok || f.Name != "Ipv4Primary" {
		t.Fatalf("fields[ipv4] = (%v, %v), want Ipv4Primary", f.Name, ok)
	}
	if f, ok := fields["shadow_ipv4"]; !ok || f.Name != "IPV4Shadow" {
		t.Fatalf("fields[shadow_ipv4] = (%v, %v), want IPV4Shadow", f.Name, ok)
	}
}

func TestExtractPublicMethods(t *testing.T) {
	keys, methods := ExtractPublicMethods(&strings.Builder{}, nil)
	if len(keys) == 0 {
		t.Error("Expected some methods from strings.Builder")
	}
	if _, ok := methods["Write"]; !ok {
		t.Error("Expected Write method")
	}
}

func TestGenerateZeroCheckExpr(t *testing.T) {
	tests := []struct {
		field    reflect.StructField
		expected string
	}{
		{reflect.StructField{Name: "Name", Type: reflect.TypeFor[string]()}, `source.Name == ""`},
		{reflect.StructField{Name: "Age", Type: reflect.TypeFor[int]()}, `source.Age == 0`},
		{reflect.StructField{Name: "Active", Type: reflect.TypeFor[bool]()}, `!source.Active`},
		{reflect.StructField{Name: "Email", Type: reflect.TypeFor[*string]()}, `source.Email == nil`},
		{reflect.StructField{Name: "Score", Type: reflect.TypeFor[float64]()}, `source.Score == 0.0`},
		{reflect.StructField{Name: "Metadata", Type: reflect.TypeFor[map[string]string]()}, `source.Metadata == nil`},
		{reflect.StructField{Name: "Flags", Type: reflect.TypeFor[[2]bool]()}, `reflect.ValueOf(source.Flags).IsZero()`},
		{reflect.StructField{Name: "Nested", Type: reflect.TypeFor[struct{ Code int }]()}, `reflect.ValueOf(source.Nested).IsZero()`},
	}

	for _, tt := range tests {
		result := GenerateZeroCheckExpr("source", tt.field)
		if result != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, result)
		}
	}
}

func TestGenerateNonZeroCheckExpr(t *testing.T) {
	tests := []struct {
		field    reflect.StructField
		expected string
	}{
		{reflect.StructField{Name: "Name", Type: reflect.TypeFor[string]()}, `source.Name != ""`},
		{reflect.StructField{Name: "Age", Type: reflect.TypeFor[int]()}, `source.Age != 0`},
		{reflect.StructField{Name: "Active", Type: reflect.TypeFor[bool]()}, `source.Active`},
		{reflect.StructField{Name: "Email", Type: reflect.TypeFor[*string]()}, `source.Email != nil`},
		{reflect.StructField{Name: "Score", Type: reflect.TypeFor[float64]()}, `source.Score != 0.0`},
		{reflect.StructField{Name: "Metadata", Type: reflect.TypeFor[map[string]string]()}, `source.Metadata != nil`},
		{reflect.StructField{Name: "Flags", Type: reflect.TypeFor[[2]bool]()}, `!reflect.ValueOf(source.Flags).IsZero()`},
		{reflect.StructField{Name: "Nested", Type: reflect.TypeFor[struct{ Code int }]()}, `!reflect.ValueOf(source.Nested).IsZero()`},
	}

	for _, tt := range tests {
		result := GenerateNonZeroCheckExpr("source", tt.field)
		if result != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, result)
		}
	}
}

func TestDeduplicateImports(t *testing.T) {
	imports := []Import{
		{Path: "fmt", Alias: ""},
		{Path: "strings", Alias: "s"},
		{Path: "fmt", Alias: ""}, // duplicate of first
		{Path: "math", Alias: ""},
	}

	result := DeduplicateImports(imports)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique imports, got %d", len(result))
	}

	// Test sorting - paths should be sorted alphabetically
	if len(result) > 1 && result[0].Path > result[1].Path {
		t.Error("Expected imports to be sorted by path")
	}

	// Verify alias is removed when it equals package name
	imports2 := []Import{
		{Path: "fmt", Alias: "fmt"},
	}
	result2 := DeduplicateImports(imports2)
	if len(result2) != 1 || result2[0].Alias != "" {
		t.Errorf("Expected alias to be removed when equals path, got %+v", result2)
	}
}

func TestDeduplicateImports_V2HyphenAndCollisions(t *testing.T) {
	imports := []Import{
		{Path: "example.com/api/user/v2", Alias: "v2"},
		{Path: "example.com/api/item/v2", Alias: "item"},
		{Path: "example.com/my-pkg", Alias: "my_pkg"},
		{Path: "example.com/a/user", Alias: "user"},
		{Path: "example.com/b/user", Alias: "user"},
		{Path: "example.com/a/user", Alias: ""},
	}
	got := DeduplicateImports(imports)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5; got=%+v", len(got), got)
	}
	byPath := make(map[string]Import, len(got))
	usedIdents := make(map[string]string, len(got))
	for _, imp := range got {
		byPath[imp.Path] = imp
		ident := imp.Alias
		if ident == "" {
			ident = fallbackPackageName(imp.Path)
		}
		if prev, dup := usedIdents[ident]; dup {
			t.Fatalf("duplicate effective identifier %q for %q and %q: %+v", ident, prev, imp.Path, got)
		}
		usedIdents[ident] = imp.Path
	}
	if byPath["example.com/api/user/v2"].Alias != "v2" {
		t.Fatalf("expected /v2 alias to be kept, got %+v", byPath["example.com/api/user/v2"])
	}
	if byPath["example.com/api/item/v2"].Alias != "item" {
		t.Fatalf("expected item alias on /v2 path to be kept, got %+v", byPath["example.com/api/item/v2"])
	}
	if byPath["example.com/my-pkg"].Alias != "my_pkg" {
		t.Fatalf("expected sanitized alias on hyphenated path, got %+v", byPath["example.com/my-pkg"])
	}
	if byPath["example.com/b/user"].Alias != "user2" {
		t.Fatalf("expected colliding user package to be disambiguated to user2, got %+v", byPath["example.com/b/user"])
	}
}

func TestGetFuncInfo_AndFallbacks(t *testing.T) {
	info := GetFuncInfo(TestGetFuncInfo_AndFallbacks)
	if info.Package != "inspect" || info.Name != "TestGetFuncInfo_AndFallbacks" {
		t.Fatalf("unexpected GetFuncInfo: %+v", info)
	}
	if !strings.HasSuffix(info.ImportPath, "/entcrud/internal/inspect") {
		t.Fatalf("unexpected ImportPath: %+v", info)
	}

	p, sym, ok := splitFuncFullName("example.com/my-pkg/v2.Convert")
	if !ok || p != "example.com/my-pkg/v2" || sym != "Convert" {
		t.Fatalf("splitFuncFullName(/v2) = (%q, %q, %v)", p, sym, ok)
	}
	p, sym, ok = splitFuncFullName("gopkg.in/foo%2ev2.Convert")
	if !ok || p != "gopkg.in/foo.v2" || sym != "Convert" {
		t.Fatalf("splitFuncFullName(escaped) = (%q, %q, %v)", p, sym, ok)
	}
	p, sym, ok = splitFuncFullName("example.com/pkg.Convert[example.com/other/sub.T]")
	if !ok || p != "example.com/pkg" || sym != "Convert[example.com/other/sub.T]" {
		t.Fatalf("splitFuncFullName(generic) = (%q, %q, %v)", p, sym, ok)
	}

	if got := fallbackPackageName("example.com/my-conv/v2"); got != "my_conv" {
		t.Fatalf("fallbackPackageName(/my-conv/v2) = %q, want my_conv", got)
	}
	if got := fallbackPackageName("example.com/pkg/v12"); got != "pkg" {
		t.Fatalf("fallbackPackageName(/pkg/v12) = %q, want pkg", got)
	}
	if got := fallbackPackageName("gopkg.in/yaml.v3"); got != "yaml" {
		t.Fatalf("fallbackPackageName(yaml.v3) = %q, want yaml", got)
	}
}

func TestGenerateTypeConversionExprWithAlias(t *testing.T) {
	type ptrTarget struct {
		Count *int32
		Meta  *strings.Builder
	}
	countFld, _ := reflect.TypeFor[ptrTarget]().FieldByName("Count")
	if got := GenerateTypeConversionExprWithAlias(countFld, reflect.TypeFor[int](), ""); got != "int(*target.Count)" {
		t.Fatalf("ptr->val expr = %q, want int(*target.Count)", got)
	}

	metaFld, _ := reflect.TypeFor[ptrTarget]().FieldByName("Meta")
	if got := GenerateTypeConversionExprWithAlias(metaFld, reflect.TypeFor[*strings.Builder](), "str2"); got != "*str2.Builder(target.Meta)" {
		t.Fatalf("ptr->ptr expr with alias = %q, want *str2.Builder(target.Meta)", got)
	}
}

func TestImportResolver_ReservesLocalsAndDisambiguates(t *testing.T) {
	r := NewImportResolver("source", "target", "options", "option")
	if got := r.Resolve("example.com/ent/option", "option"); got != "option2" {
		t.Fatalf("expected reserved option to disambiguate to option2, got %q", got)
	}
	if got := r.Resolve("example.com/a/user", "user"); got != "user" {
		t.Fatalf("expected user, got %q", got)
	}
	if got := r.Resolve("example.com/b/user", "user"); got != "user2" {
		t.Fatalf("expected colliding user to become user2, got %q", got)
	}
}
