package converter

import (
	"errors"
	"path/filepath"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

// TestNewConverter_RejectsUnrenderableCustomGoType guards the regression where
// custom Go types (driver.Valuer / sql.Scanner / BinaryMarshaler) produced a
// converter the template could not render, silently degrading to an identity
// assignment that does not compile.
func TestNewConverter_RejectsUnrenderableCustomGoType(t *testing.T) {
	schemaDir := filepath.Join("..", "..", "testdata", "fixtures", "gotypeschema")
	g, err := entc.LoadGraph(schemaDir, &gen.Config{IDType: &field.TypeInfo{Type: field.TypeInt64}})
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	adapter, err := entproto.LoadAdapter(g)
	if err != nil {
		t.Fatalf("load adapter: %v", err)
	}
	fm, err := adapter.FieldMap("User")
	if err != nil {
		t.Fatalf("field map: %v", err)
	}
	mapping := fm["nick"]
	if mapping == nil {
		t.Fatalf("field nick missing from map %v", fm)
	}
	_, err = NewConverter(mapping, "User")
	if err == nil {
		t.Fatal("expected UnsupportedEntGoTypeError for sql.NullString field")
	}
	var ue *UnsupportedEntGoTypeError
	if !errors.As(err, &ue) {
		t.Fatalf("expected *UnsupportedEntGoTypeError, got %T (%v)", err, err)
	}
}
