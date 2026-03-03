package inspect

import (
	"reflect"
	"strings"
	"testing"
)

type testStruct struct {
	Name     string
	Age      int
	Email    *string
	Active   bool
	Score    float64
	Tags     []string
	Metadata map[string]string
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
	if len(keys) != 7 {
		t.Errorf("Expected 7 fields, got %d", len(keys))
	}
	if _, ok := fields["Name"]; !ok {
		t.Error("Expected Name field")
	}
	if _, ok := fields["Email"]; !ok {
		t.Error("Expected Email field")
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
