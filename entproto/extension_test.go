package entproto

import (
	"io"
	"os"
	"strings"
	"testing"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
)

func TestSkippedSchemasWarning(t *testing.T) {
	got := skippedSchemasWarning([]string{"User", "Group"})
	if !strings.Contains(got, "Group, User") {
		t.Fatalf("warning does not list all schemas in sorted order: %s", got)
	}
	if !strings.Contains(got, "entproto.Message") {
		t.Fatalf("warning does not mention the missing annotation: %s", got)
	}
}

func TestSkippedSchemasWarning_SortsInput(t *testing.T) {
	got := skippedSchemasWarning([]string{"Zeta", "Alpha"})
	if !strings.Contains(got, "Alpha, Zeta") {
		t.Fatalf("warning is not sorted: %s", got)
	}
}

func TestGenerate_ReportsFailingSchemaAndWarnsOnlyForUnannotated(t *testing.T) {
	g := &gen.Graph{
		Nodes: []*gen.Type{
			testType("Bad", Message(), []*gen.Field{{
				Name: "blob", Type: &field.TypeInfo{Type: field.TypeJSON, Ident: "map[string]any"},
				Annotations: map[string]any{FieldAnnotation: Field(2)},
			}}, nil),
		},
		Config: &gen.Config{Package: "example.com/app/ent", Target: t.TempDir()},
	}
	err := Generate(g)
	if err == nil || !strings.Contains(err.Error(), "schema Bad") {
		t.Fatalf("Generate error = %v, want it to name schema Bad", err)
	}

	unannotated := testType("Plain", nil, nil, nil)
	g = &gen.Graph{
		Nodes:  []*gen.Type{testType("Hidden", SkipGen(), nil, nil), unannotated},
		Config: &gen.Config{Package: "example.com/app/ent", Target: t.TempDir()},
	}
	stderr := captureStderr(t, func() {
		if err := Generate(g); err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
	})
	if !strings.Contains(stderr, "Plain") || strings.Contains(stderr, "Hidden") {
		t.Fatalf("warning = %q, want it to list Plain but not the SkipGen schema Hidden", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
