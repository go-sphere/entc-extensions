package entproto

import (
	"errors"
	"testing"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
)

func TestFixGraph_InvalidAnnotationReturnsError(t *testing.T) {
	g := &gen.Graph{
		Nodes: []*gen.Type{
			{
				Name: "User",
				ID: &gen.Field{
					Name: "id",
					Type: &field.TypeInfo{Type: field.TypeInt},
				},
				Fields: []*gen.Field{
					{
						Name:        "name",
						Type:        &field.TypeInfo{Type: field.TypeString},
						Annotations: map[string]any{FieldAnnotation: map[string]any{"Number": "bad"}},
					},
				},
			},
		},
	}

	err := FixGraph(g)
	if err == nil {
		t.Fatal("expected invalid annotation error")
	}
	if !errors.Is(err, ErrInvalidAnnotation) {
		t.Fatalf("expected ErrInvalidAnnotation, got %v", err)
	}
	var invalid *InvalidAnnotationError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected *InvalidAnnotationError, got %T", err)
	}
	if invalid.Schema != "User" || invalid.Field != "name" {
		t.Fatalf("invalid annotation location = %s.%s", invalid.Schema, invalid.Field)
	}
}

func TestFixGraph_AutoNumbersRemainStableAcrossInsertion(t *testing.T) {
	first := autoFillGraph("alpha", "beta")
	if err := FixGraph(first); err != nil {
		t.Fatalf("first FixGraph failed: %v", err)
	}
	firstNumbers := fieldNumbers(t, first.Nodes[0])

	second := autoFillGraph("new_field", "alpha", "beta")
	if err := FixGraph(second); err != nil {
		t.Fatalf("second FixGraph failed: %v", err)
	}
	secondNumbers := fieldNumbers(t, second.Nodes[0])

	for _, name := range []string{"alpha", "beta"} {
		if firstNumbers[name] != secondNumbers[name] {
			t.Fatalf("field %q number changed from %d to %d", name, firstNumbers[name], secondNumbers[name])
		}
	}
	if secondNumbers["new_field"] == firstNumbers["alpha"] || secondNumbers["new_field"] == firstNumbers["beta"] {
		t.Fatalf("new field reused an existing number: %v", secondNumbers)
	}
}

func TestFixGraph_ContinuesAfterExistingMessageAndIDAnnotations(t *testing.T) {
	g := autoFillGraph("name")
	node := g.Nodes[0]
	node.Annotations = map[string]any{MessageAnnotation: Message(PackageName("acme.user.v1"))}
	node.ID.Annotations = map[string]any{FieldAnnotation: Field(IDFieldNumber)}

	if err := FixGraph(g); err != nil {
		t.Fatalf("FixGraph failed: %v", err)
	}
	msg, err := extractMessageAnnotation(node)
	if err != nil {
		t.Fatalf("extract message annotation: %v", err)
	}
	if msg.Package != "acme.user.v1" {
		t.Fatalf("message package = %q, want acme.user.v1", msg.Package)
	}
	numbers := fieldNumbers(t, node)
	if numbers["id"] != IDFieldNumber {
		t.Fatalf("ID number = %d, want %d", numbers["id"], IDFieldNumber)
	}
	if numbers["name"] == IDFieldNumber {
		t.Fatal("auto-filled field reused the ID number")
	}
}

func TestFixGraph_StableNumberCollisionRequiresExplicitAnnotation(t *testing.T) {
	collision := stableFieldNumber("User", "field", "name")
	g := autoFillGraph("existing", "name")
	g.Nodes[0].Fields[0].Annotations = map[string]any{FieldAnnotation: Field(collision)}

	if err := FixGraph(g); err == nil {
		t.Fatal("expected stable number collision to fail")
	}
}

func TestAdapter_DuplicateFieldNumberReturnsError(t *testing.T) {
	node := &gen.Type{
		Name: "User",
		ID: &gen.Field{
			Name:        "id",
			Type:        &field.TypeInfo{Type: field.TypeInt64},
			UserDefined: true,
			Annotations: map[string]any{FieldAnnotation: Field(1)},
		},
		Fields: []*gen.Field{
			{
				Name:        "name",
				Type:        &field.TypeInfo{Type: field.TypeString},
				Annotations: map[string]any{FieldAnnotation: Field(2)},
			},
			{
				Name:        "email",
				Type:        &field.TypeInfo{Type: field.TypeString},
				Annotations: map[string]any{FieldAnnotation: Field(2)},
			},
		},
		Annotations: map[string]any{MessageAnnotation: Message()},
	}
	a := &Adapter{nodeByName: map[string]*gen.Type{"User": node}}

	_, err := a.toProtoMessageDescriptor(node)
	if err == nil {
		t.Fatal("expected duplicate field number error")
	}
	if !errors.Is(err, ErrDuplicateFieldNumber) {
		t.Fatalf("expected ErrDuplicateFieldNumber, got %v", err)
	}
	var duplicate *DuplicateFieldNumberError
	if !errors.As(err, &duplicate) {
		t.Fatalf("expected *DuplicateFieldNumberError, got %T", err)
	}
	if duplicate.Number != 2 {
		t.Fatalf("duplicate number = %d, want 2", duplicate.Number)
	}
}

func autoFillGraph(fieldNames ...string) *gen.Graph {
	fields := make([]*gen.Field, 0, len(fieldNames))
	for _, name := range fieldNames {
		fields = append(fields, &gen.Field{
			Name: name,
			Type: &field.TypeInfo{Type: field.TypeString},
		})
	}
	return &gen.Graph{Nodes: []*gen.Type{{
		Name: "User",
		ID: &gen.Field{
			Name: "id",
			Type: &field.TypeInfo{Type: field.TypeInt64},
		},
		Fields: fields,
	}}}
}

func fieldNumbers(t *testing.T, node *gen.Type) map[string]int {
	t.Helper()
	result := make(map[string]int, len(node.Fields)+1)
	fields := append([]*gen.Field{node.ID}, node.Fields...)
	for _, fld := range fields {
		annotation, err := extractFieldAnnotation(fld)
		if err != nil {
			t.Fatalf("extract field annotation for %s: %v", fld.Name, err)
		}
		result[fld.Name] = annotation.Number
	}
	return result
}
