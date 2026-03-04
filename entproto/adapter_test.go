package entproto

import (
	"path/filepath"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
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
