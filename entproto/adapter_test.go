package entproto

import (
	"path/filepath"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func TestLoadAdapter(t *testing.T) {
	graph, err := entc.LoadGraph("./internal/todo/ent/schema", &gen.Config{})
	if err != nil {
		t.Fatalf("LoadGraph failed: %v", err)
	}

	adapter, err := LoadAdapter(graph)
	if err != nil {
		t.Fatalf("LoadAdapter failed: %v", err)
	}

	// Test User message
	fd, err := adapter.GetFileDescriptor("User")
	if err != nil {
		t.Fatalf("GetFileDescriptor failed: %v", err)
	}
	if got, want := fd.GetName(), filepath.Join("entpb", "entpb.proto"); got != want {
		t.Errorf("fd.GetName() = %v, want %v", got, want)
	}

	msg := fd.FindMessage("entpb.User")
	if msg == nil {
		t.Fatal("msg is nil")
	}

	idField := msg.FindFieldByName("id")
	if idField == nil {
		t.Fatal("idField is nil")
	}
	if idField.GetNumber() != 1 {
		t.Errorf("idField.GetNumber() = %v, want 1", idField.GetNumber())
	}

	nameField := msg.FindFieldByName("name")
	if nameField == nil {
		t.Fatal("nameField is nil")
	}
	if nameField.GetNumber() != 2 {
		t.Errorf("nameField.GetNumber() = %v, want 2", nameField.GetNumber())
	}
	if nameField.GetType().String() != "TYPE_STRING" {
		t.Errorf("nameField.GetType() = %v, want TYPE_STRING", nameField.GetType())
	}

	ageField := msg.FindFieldByName("age")
	if ageField == nil {
		t.Fatal("ageField is nil")
	}
	if ageField.GetNumber() != 3 {
		t.Errorf("ageField.GetNumber() = %v, want 3", ageField.GetNumber())
	}

	activeField := msg.FindFieldByName("active")
	if activeField == nil {
		t.Fatal("activeField is nil")
	}
	if activeField.GetNumber() != 4 {
		t.Errorf("activeField.GetNumber() = %v, want 4", activeField.GetNumber())
	}

	// Test Post message
	fd, err = adapter.GetFileDescriptor("Post")
	if err != nil {
		t.Fatalf("GetFileDescriptor failed: %v", err)
	}

	msg = fd.FindMessage("entpb.Post")
	if msg == nil {
		t.Fatal("msg is nil")
	}

	titleField := msg.FindFieldByName("title")
	if titleField == nil {
		t.Fatal("titleField is nil")
	}
	if titleField.GetNumber() != 2 {
		t.Errorf("titleField.GetNumber() = %v, want 2", titleField.GetNumber())
	}

	authorField := msg.FindFieldByName("author")
	if authorField == nil {
		t.Fatal("authorField is nil")
	}
	if authorField.GetNumber() != 4 {
		t.Errorf("authorField.GetNumber() = %v, want 4", authorField.GetNumber())
	}

	// Test Task message (enum)
	fd, err = adapter.GetFileDescriptor("Task")
	if err != nil {
		t.Fatalf("GetFileDescriptor failed: %v", err)
	}

	msg = fd.FindMessage("entpb.Task")
	if msg == nil {
		t.Fatal("msg is nil")
	}

	statusField := msg.FindFieldByName("status")
	if statusField == nil {
		t.Fatal("statusField is nil")
	}
	if statusField.GetNumber() != 3 {
		t.Errorf("statusField.GetNumber() = %v, want 3", statusField.GetNumber())
	}
	if statusField.GetType().String() != "TYPE_ENUM" {
		t.Errorf("statusField.GetType() = %v, want TYPE_ENUM", statusField.GetType())
	}

	enumType := statusField.GetEnumType()
	if enumType == nil {
		t.Fatal("enumType is nil")
	}
	if enumType.GetFullyQualifiedName() != "entpb.Task.Status" {
		t.Errorf("enumType.GetFullyQualifiedName() = %v, want entpb.Task.Status", enumType.GetFullyQualifiedName())
	}
}

func TestSkipMessage(t *testing.T) {
	graph, err := entc.LoadGraph("./internal/todo/ent/schema", &gen.Config{})
	if err != nil {
		t.Fatalf("LoadGraph failed: %v", err)
	}

	adapter, err := LoadAdapter(graph)
	if err != nil {
		t.Fatalf("LoadAdapter failed: %v", err)
	}

	// Test that edges are converted to fields
	fd, err := adapter.GetFileDescriptor("User")
	if err != nil {
		t.Fatalf("GetFileDescriptor failed: %v", err)
	}

	msg := fd.FindMessage("entpb.User")
	if msg == nil {
		t.Fatal("msg is nil")
	}

	// Check edge field
	postsField := msg.FindFieldByName("posts")
	if postsField == nil {
		t.Fatal("postsField is nil")
	}
	if postsField.GetNumber() != 5 {
		t.Errorf("postsField.GetNumber() = %v, want 5", postsField.GetNumber())
	}
	if postsField.GetType().String() != "TYPE_MESSAGE" {
		t.Errorf("postsField.GetType() = %v, want TYPE_MESSAGE", postsField.GetType())
	}
}

func TestAutoFill(t *testing.T) {
	graph, err := entc.LoadGraph("./internal/todo/ent/schema", &gen.Config{})
	if err != nil {
		t.Fatalf("LoadGraph failed: %v", err)
	}

	// Apply auto-fill to graph
	FixGraph(graph)

	adapter, err := LoadAdapter(graph)
	if err != nil {
		t.Fatalf("LoadAdapter failed: %v", err)
	}

	// Test Item schema which has no entproto annotations
	// After FixGraph, it should have auto-generated annotations
	fd, err := adapter.GetFileDescriptor("Item")
	if err != nil {
		t.Fatalf("GetFileDescriptor failed: %v", err)
	}
	if fd.GetName() != filepath.Join("entpb", "entpb.proto") {
		t.Errorf("fd.GetName() = %v, want %v", fd.GetName(), filepath.Join("entpb", "entpb.proto"))
	}

	msg := fd.FindMessage("entpb.Item")
	if msg == nil {
		t.Fatal("msg is nil")
	}

	idField := msg.FindFieldByName("id")
	if idField == nil {
		t.Fatal("idField is nil")
	}
	if idField.GetNumber() != 1 {
		t.Errorf("idField.GetNumber() = %v, want 1", idField.GetNumber())
	}

	nameField := msg.FindFieldByName("name")
	if nameField == nil {
		t.Fatal("nameField is nil")
	}
	if nameField.GetNumber() != 2 {
		t.Errorf("nameField.GetNumber() = %v, want 2", nameField.GetNumber())
	}
	if nameField.GetType().String() != "TYPE_STRING" {
		t.Errorf("nameField.GetType() = %v, want TYPE_STRING", nameField.GetType())
	}

	quantityField := msg.FindFieldByName("quantity")
	if quantityField == nil {
		t.Fatal("quantityField is nil")
	}
	if quantityField.GetNumber() != 3 {
		t.Errorf("quantityField.GetNumber() = %v, want 3", quantityField.GetNumber())
	}
	if quantityField.GetType().String() != "TYPE_INT64" {
		t.Errorf("quantityField.GetType() = %v, want TYPE_INT64", quantityField.GetType())
	}

	availableField := msg.FindFieldByName("available")
	if availableField == nil {
		t.Fatal("availableField is nil")
	}
	if availableField.GetNumber() != 4 {
		t.Errorf("availableField.GetNumber() = %v, want 4", availableField.GetNumber())
	}
	if availableField.GetType().String() != "TYPE_BOOL" {
		t.Errorf("availableField.GetType() = %v, want TYPE_BOOL", availableField.GetType())
	}
}
