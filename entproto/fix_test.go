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

func TestFieldIDGenerator_OverflowReturnsError(t *testing.T) {
	g := &fieldIDGenerator{
		schema:  "User",
		current: 536870911,
		exist:   map[int]struct{}{},
	}
	_, err := g.Next("name")
	if err == nil {
		t.Fatal("expected overflow error")
	}
	if !errors.Is(err, ErrFieldNumberOverflow) {
		t.Fatalf("expected ErrFieldNumberOverflow, got %v", err)
	}
	var overflow *FieldNumberOverflowError
	if !errors.As(err, &overflow) {
		t.Fatalf("expected *FieldNumberOverflowError, got %T", err)
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
