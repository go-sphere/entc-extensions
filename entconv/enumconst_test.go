package entconv

import (
	"path/filepath"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

// TestPbEnumValueConstName_ResolvesRealDescriptor proves the constant name is
// read from the real protobuf enum descriptor rather than re-derived. For a
// field named ip_v4 the enum type is IPV4 while its values are IP_V4_*, so the
// actual protoc-gen-go constant is User_IP_V4_LOW, not User_IPV4_LOW.
func TestPbEnumValueConstName_ResolvesRealDescriptor(t *testing.T) {
	schema := filepath.Join(moduleRoot(t), "testdata", "fixtures", "enumschema")
	g, err := entc.LoadGraph(schema, &gen.Config{IDType: &field.TypeInfo{Type: field.TypeInt64}})
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

	var mapping *entproto.FieldMappingDescriptor
	for _, m := range fm {
		if m.IsEnumField {
			mapping = m
			break
		}
	}
	if mapping == nil {
		t.Fatalf("no enum mapping found in %v", fm)
	}

	cases := []struct {
		value string
		want  string
	}{
		{"low", "User_IP_V4_LOW"},
		{"high", "User_IP_V4_HIGH"},
	}
	for _, c := range cases {
		got, err := mapping.PbEnumValueConstName(c.value)
		if err != nil {
			t.Fatalf("PbEnumValueConstName(%q): %v", c.value, err)
		}
		if got != c.want {
			t.Errorf("PbEnumValueConstName(%q) = %q, want %q", c.value, got, c.want)
		}
	}
}
