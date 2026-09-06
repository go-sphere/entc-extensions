package entconv

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
)

func TestStructIDType(t *testing.T) {
	dir := t.TempDir()
	content := `package ent

type User struct {
	ID int64 ` + "`json:\"id,omitempty\"`" + `
	Name string
}
`
	if err := os.WriteFile(filepath.Join(dir, "user.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	typ, ok := structIDType(dir, "User")
	if !ok {
		t.Fatal("expected ID field to be found")
	}
	if typ != "int64" {
		t.Fatalf("ID type = %q, want int64", typ)
	}
}

func TestStructIDType_MissingTypeReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "user.go"), []byte("package ent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := structIDType(dir, "User"); ok {
		t.Fatal("expected no ID field for absent type")
	}
}

func TestVerifyIDTypesAgainstSource_NoEntDirIsNoop(t *testing.T) {
	// schema dir with no sibling "ent" directory: check passes silently.
	schemaDir := t.TempDir()
	g := &gen.Graph{Nodes: []*gen.Type{{
		Name: "User",
		ID:   &gen.Field{Type: &field.TypeInfo{Type: field.TypeInt64}},
	}}}
	if err := verifyIDTypesAgainstSource(g, schemaDir); err != nil {
		t.Fatalf("expected no-op when ent source is missing, got %v", err)
	}
}

func TestVerifyIDTypesAgainstSource_MismatchReturnsError(t *testing.T) {
	root := t.TempDir()
	// Layout: <root>/schema/<schemas> and <root>/ent/<generated ent structs>.
	schemaDir := filepath.Join(root, "schema")
	entDir := filepath.Join(root, "ent")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(entDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Real generated ent struct has an int ID...
	content := "package ent\n\ntype User struct {\n\tID int `json:\"id,omitempty\"`\n}\n"
	if err := os.WriteFile(filepath.Join(entDir, "user.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// ...but entconv assumed int64 (e.g. default IDType option).
	g := &gen.Graph{Nodes: []*gen.Type{{
		Name: "User",
		ID:   &gen.Field{Type: &field.TypeInfo{Type: field.TypeInt64}},
	}}}
	err := verifyIDTypesAgainstSource(g, schemaDir)
	if err == nil {
		t.Fatal("expected IDTypeMismatchError for int64 assumed vs int actual")
	}
	var mismatch *IDTypeMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected *IDTypeMismatchError, got %T (%v)", err, err)
	}
	if mismatch.Entity != "User" || mismatch.ActualType != "int" || mismatch.ExpectedType != "int64" {
		t.Fatalf("mismatch = %+v, want User int<-int64", mismatch)
	}
}

func TestVerifyIDTypesAgainstSource_MatchPasses(t *testing.T) {
	root := t.TempDir()
	schemaDir := filepath.Join(root, "schema")
	entDir := filepath.Join(root, "ent")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(entDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "package ent\n\ntype User struct {\n\tID int64 `json:\"id,omitempty\"`\n}\n"
	if err := os.WriteFile(filepath.Join(entDir, "user.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &gen.Graph{Nodes: []*gen.Type{{
		Name: "User",
		ID:   &gen.Field{Type: &field.TypeInfo{Type: field.TypeInt64}},
	}}}
	if err := verifyIDTypesAgainstSource(g, schemaDir); err != nil {
		t.Fatalf("expected match to pass, got %v", err)
	}
}

func TestIDTypeNamesEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"int64", "int64", true},
		{"int", "int64", false},
		{"uuid.UUID", "UUID", true},
		{"[]byte", "[]byte", true},
	}
	for _, c := range cases {
		if got := idTypeNamesEqual(c.a, c.b); got != c.want {
			t.Errorf("idTypeNamesEqual(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
