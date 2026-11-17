package generator

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/entc/load"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

func TestRemoveStaleConverters(t *testing.T) {
	dir := t.TempDir()

	generated := filepath.Join(dir, "stale.go")
	if err := os.WriteFile(generated, []byte(generatedHeader+"\npackage entmap\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handwritten := filepath.Join(dir, "custom.go")
	if err := os.WriteFile(handwritten, []byte("package entmap\n\nfunc Custom() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nonGo := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(nonGo, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeStaleConverters(dir); err != nil {
		t.Fatalf("removeStaleConverters: %v", err)
	}

	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("generated file should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(handwritten); err != nil {
		t.Fatalf("handwritten file must be kept: %v", err)
	}
	if _, err := os.Stat(nonGo); err != nil {
		t.Fatalf("non-Go file must be kept: %v", err)
	}
}

func buildTestGraph(t *testing.T, pkg string, schemas ...*load.Schema) *gen.Graph {
	t.Helper()
	g, err := gen.NewGraph(&gen.Config{
		Package: pkg,
		IDType:  &field.TypeInfo{Type: field.TypeInt64},
	}, schemas...)
	if err != nil {
		t.Fatalf("gen.NewGraph: %v", err)
	}
	return g
}

func TestGenerate_MultiWordSchemaEnumUsesPackageDirNotCamel(t *testing.T) {
	g := buildTestGraph(t, "example.com/app/ent", &load.Schema{
		Name:        "BlogPost",
		Annotations: map[string]any{entproto.MessageAnnotation: entproto.Message()},
		Fields: []*load.Field{{
			Name:  "status",
			Info:  &field.TypeInfo{Type: field.TypeEnum},
			Enums: []struct{ N, V string }{{N: "Draft", V: "draft"}, {N: "Live", V: "live"}},
			Annotations: map[string]any{
				entproto.FieldAnnotation: entproto.Field(2),
				entproto.EnumAnnotation:  entproto.Enum(map[string]int32{"draft": 1, "live": 2}),
			},
		}},
	})
	adapter, err := entproto.LoadAdapter(g)
	if err != nil {
		t.Fatalf("LoadAdapter: %v", err)
	}
	gen := New("example.com/app/ent", "entmap", "example.com/app/entpb", "entpb", []TypeInfo{{
		MessageName: "BlogPost",
		Message:     &ProtoMessage{Name: "BlogPost"},
		Type:        g.Nodes[0],
	}}, adapter, g)

	var buf bytes.Buffer
	if err := gen.GenerateToWriter(&buf); err != nil {
		t.Fatalf("GenerateToWriter: %v", err)
	}
	code := buf.String()
	if !strings.Contains(code, `blogpost "example.com/app/ent/blogpost"`) {
		t.Fatalf("expected blogpost subpackage import, got:\n%s", code)
	}
	if strings.Contains(code, "blogPost.") {
		t.Fatalf("generated code used camelCased package name blogPost:\n%s", code)
	}
	if !strings.Contains(code, "blogpost.Status") {
		t.Fatalf("expected blogpost.Status reference, got:\n%s", code)
	}
	if _, err := format.Source(buf.Bytes()); err != nil {
		t.Fatalf("generated code does not format cleanly: %v\n%s", err, code)
	}
}

func TestGenerate_DisambiguatesCollidingImportAliases(t *testing.T) {
	g := buildTestGraph(t, "example.com/app/ent", &load.Schema{
		Name:        "Time",
		Annotations: map[string]any{entproto.MessageAnnotation: entproto.Message()},
		Fields: []*load.Field{
			{
				Name:        "created_at",
				Info:        &field.TypeInfo{Type: field.TypeTime},
				Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(2)},
			},
			{
				Name:  "kind",
				Info:  &field.TypeInfo{Type: field.TypeEnum},
				Enums: []struct{ N, V string }{{N: "A", V: "a"}},
				Annotations: map[string]any{
					entproto.FieldAnnotation: entproto.Field(3),
					entproto.EnumAnnotation:  entproto.Enum(map[string]int32{"a": 1}),
				},
			},
			{
				Name:        "custom_time",
				Info:        &field.TypeInfo{Type: field.TypeString, Ident: "time.Label", PkgPath: "example.com/custom/time", PkgName: "time", RType: &field.RType{Ident: "time.Label", PkgPath: "example.com/custom/time"}},
				Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(4)},
			},
			{
				Name:        "custom_ent",
				Info:        &field.TypeInfo{Type: field.TypeString, Ident: "ent.Tag", PkgPath: "example.com/custom/ent", PkgName: "ent", RType: &field.RType{Ident: "ent.Tag", PkgPath: "example.com/custom/ent"}},
				Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(5)},
			},
			{
				Name:        "dup_a",
				Info:        &field.TypeInfo{Type: field.TypeString, Ident: "shared.A", PkgPath: "example.com/a/shared", PkgName: "shared", RType: &field.RType{Ident: "shared.A", PkgPath: "example.com/a/shared"}},
				Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(6)},
			},
			{
				Name:        "dup_b",
				Info:        &field.TypeInfo{Type: field.TypeString, Ident: "shared.B", PkgPath: "example.com/b/shared", PkgName: "shared", RType: &field.RType{Ident: "shared.B", PkgPath: "example.com/b/shared"}},
				Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(7)},
			},
		},
	})
	adapter, err := entproto.LoadAdapter(g)
	if err != nil {
		t.Fatalf("LoadAdapter: %v", err)
	}
	// Deliberately pass ProtoAlias="time" to collide with stdlib time.
	gen := New("example.com/app/ent", "entmap", "example.com/app/pb", "time", []TypeInfo{{
		MessageName: "Time",
		Message:     &ProtoMessage{Name: "Time"},
		Type:        g.Nodes[0],
	}}, adapter, g)

	var buf bytes.Buffer
	if err := gen.GenerateToWriter(&buf); err != nil {
		t.Fatalf("GenerateToWriter: %v", err)
	}
	code := buf.String()
	f, err := parser.ParseFile(token.NewFileSet(), "time.go", code, 0)
	if err != nil {
		t.Fatalf("generated code failed to parse: %v\n%s", err, code)
	}
	seenAliases := make(map[string]string)
	for _, imp := range f.Imports {
		pathVal, _ := strconv.Unquote(imp.Path.Value)
		alias := filepath.Base(pathVal)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if prev, dup := seenAliases[alias]; dup {
			t.Fatalf("duplicate import alias %q for %q and %q\n%s", alias, prev, pathVal, code)
		}
		seenAliases[alias] = pathVal
	}
	if !strings.Contains(code, "time.Unix(v.CreatedAt, 0)") {
		t.Fatalf("expected stdlib time.Unix call, got:\n%s", code)
	}
	if !strings.Contains(code, "time2.Time") {
		t.Fatalf("expected disambiguated proto alias time2.Time, got:\n%s", code)
	}
	if !strings.Contains(code, "time3.Label(v.CustomTime)") {
		t.Fatalf("expected rewritten GoType qualifier time3.Label, got:\n%s", code)
	}
	if !strings.Contains(code, "ent2.Tag(v.CustomEnt)") {
		t.Fatalf("expected rewritten GoType qualifier ent2.Tag, got:\n%s", code)
	}
	if !strings.Contains(code, "shared2.B(v.DupB)") {
		t.Fatalf("expected rewritten GoType qualifier shared2.B, got:\n%s", code)
	}
}

func TestGenerate_AvoidsLocalVariableShadowing(t *testing.T) {
	g := buildTestGraph(t, "example.com/app/ent", &load.Schema{
		Name:        "Shadow",
		Annotations: map[string]any{entproto.MessageAnnotation: entproto.Message()},
		Fields: []*load.Field{
			{Name: "e", Info: &field.TypeInfo{Type: field.TypeString}, Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(2)}},
			{Name: "v", Info: &field.TypeInfo{Type: field.TypeString}, Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(3)}},
			{Name: "ent", Info: &field.TypeInfo{Type: field.TypeString}, Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(4)}},
			{Name: "time", Info: &field.TypeInfo{Type: field.TypeTime}, Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(5)}},
			{Name: "entpb", Info: &field.TypeInfo{Type: field.TypeString}, Annotations: map[string]any{entproto.FieldAnnotation: entproto.Field(6)}},
		},
	})
	adapter, err := entproto.LoadAdapter(g)
	if err != nil {
		t.Fatalf("LoadAdapter: %v", err)
	}
	gen := New("example.com/app/ent", "entmap", "example.com/app/entpb", "entpb", []TypeInfo{{
		MessageName: "Shadow",
		Message:     &ProtoMessage{Name: "Shadow"},
		Type:        g.Nodes[0],
	}}, adapter, g)

	var buf bytes.Buffer
	if err := gen.GenerateToWriter(&buf); err != nil {
		t.Fatalf("GenerateToWriter: %v", err)
	}
	code := buf.String()
	for _, shadowed := range []string{"\te := ", "\tv := e.", "\tent := ", "\ttime := ", "\tentpb := "} {
		if strings.Contains(code, shadowed) {
			t.Fatalf("generated code contains shadowing declaration %q:\n%s", shadowed, code)
		}
	}
	for _, want := range []string{"field_e := e.E", "field_v := e.V", "field_ent := e.Ent", "field_time := e.Time.Unix()", "field_entpb := e.Entpb"} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated code missing safe local var %q:\n%s", want, code)
		}
	}
	if _, err := format.Source(buf.Bytes()); err != nil {
		t.Fatalf("generated code failed gofmt: %v\n%s", err, code)
	}
}
