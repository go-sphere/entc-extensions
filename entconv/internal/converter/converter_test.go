package converter

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
	"github.com/jhump/protoreflect/desc" //nolint:staticcheck
	"github.com/jhump/protoreflect/desc/builder"
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

type binaryID struct{}

func (binaryID) MarshalBinary() ([]byte, error) { return nil, nil }
func (*binaryID) UnmarshalBinary([]byte) error  { return nil }

type valuerInt int32

func (v valuerInt) Value() (driver.Value, error) { return int64(v), nil }

func buildTestFieldDesc(t *testing.T, name string, ft *builder.FieldType, repeated bool) *desc.FieldDescriptor {
	t.Helper()
	fb := builder.NewField(name, ft).SetNumber(2)
	if repeated {
		fb.SetRepeated()
	}
	msg, err := builder.NewMessage("Msg").AddField(fb).Build()
	if err != nil {
		t.Fatalf("build message descriptor for %s: %v", name, err)
	}
	return msg.FindFieldByName(name)
}

func TestNewConverter_DoubleValuerFailsAndFloat32ToDoubleConverts(t *testing.T) {
	pbd := buildTestFieldDesc(t, "score", builder.FieldTypeDouble(), false)

	valuerField := &gen.Field{
		Name: "score",
		Type: field.Float("score").GoType(sql.NullFloat64{}).Descriptor().Info,
	}
	_, err := NewConverter(&entproto.FieldMappingDescriptor{
		EntField:          valuerField,
		PbFieldDescriptor: pbd,
	}, "User")
	var ue *UnsupportedEntGoTypeError
	if !errors.As(err, &ue) {
		t.Fatalf("expected *UnsupportedEntGoTypeError for Valuer double, got %T (%v)", err, err)
	}

	f32Field := &gen.Field{
		Name: "score",
		Type: &field.TypeInfo{Type: field.TypeFloat32},
	}
	conv, err := NewConverter(&entproto.FieldMappingDescriptor{
		EntField:          f32Field,
		PbFieldDescriptor: pbd,
	}, "User")
	if err != nil {
		t.Fatalf("NewConverter float32->double failed: %v", err)
	}
	if conv.ToProtoConversion != "float64" || conv.ToEntConversion != "float32" {
		t.Fatalf("unexpected conversions: %+v", conv)
	}
}

func TestNewConverter_NumericValuerFails(t *testing.T) {
	fieldTypes := []*builder.FieldType{
		builder.FieldTypeInt32(),
		builder.FieldTypeSInt32(),
		builder.FieldTypeSFixed32(),
		builder.FieldTypeInt64(),
		builder.FieldTypeSInt64(),
		builder.FieldTypeSFixed64(),
		builder.FieldTypeUInt32(),
		builder.FieldTypeFixed32(),
		builder.FieldTypeUInt64(),
		builder.FieldTypeFixed64(),
		builder.FieldTypeFloat(),
		builder.FieldTypeDouble(),
	}
	valuerInfo := field.Int32("v").GoType(valuerInt(0)).Descriptor().Info
	for _, ft := range fieldTypes {
		pbd := buildTestFieldDesc(t, "v", ft, false)
		_, err := NewConverter(&entproto.FieldMappingDescriptor{
			EntField:          &gen.Field{Name: "v", Type: valuerInfo},
			PbFieldDescriptor: pbd,
		}, "User")
		var ue *UnsupportedEntGoTypeError
		if !errors.As(err, &ue) {
			t.Fatalf("type %v: expected *UnsupportedEntGoTypeError, got %T (%v)", ft.GetType(), err, err)
		}
	}
}

func TestNewConverter_IDWithBinaryMarshalerOrValueScannerFails(t *testing.T) {
	bytesDesc := buildTestFieldDesc(t, "id", builder.FieldTypeBytes(), false)
	binField := &gen.Field{
		Name: "id",
		Type: field.Bytes("id").GoType(&binaryID{}).Descriptor().Info,
	}
	_, err := NewConverter(&entproto.FieldMappingDescriptor{
		EntField:          binField,
		PbFieldDescriptor: bytesDesc,
		IsIDField:         true,
	}, "User")
	var ue *UnsupportedEntGoTypeError
	if !errors.As(err, &ue) {
		t.Fatalf("BinaryMarshaler ID: expected *UnsupportedEntGoTypeError, got %T (%v)", err, err)
	}

	strDesc := buildTestFieldDesc(t, "id", builder.FieldTypeString(), false)
	scannerField := &gen.Field{
		Name: "id",
		Type: field.String("id").GoType(sql.NullString{}).Descriptor().Info,
	}
	_, err = NewConverter(&entproto.FieldMappingDescriptor{
		EntField:          scannerField,
		PbFieldDescriptor: strDesc,
		IsIDField:         true,
	}, "User")
	if !errors.As(err, &ue) {
		t.Fatalf("ValueScanner ID: expected *UnsupportedEntGoTypeError, got %T (%v)", err, err)
	}
}

func TestNewConverter_NilEnumTypeOrNilEdgeIDDescReturnsError(t *testing.T) {
	enumField := &gen.Field{
		Name: "status",
		Type: &field.TypeInfo{Type: field.TypeEnum},
	}
	strPbd := buildTestFieldDesc(t, "status", builder.FieldTypeString(), false)
	if _, err := NewConverter(&entproto.FieldMappingDescriptor{
		EntField:          enumField,
		PbFieldDescriptor: strPbd,
	}, "User"); err == nil || !strings.Contains(err.Error(), "no enum descriptor") {
		t.Fatalf("expected missing enum descriptor error, got %v", err)
	}

	target := &gen.Type{
		Name: "Target",
		ID:   &gen.Field{Name: "id", Type: &field.TypeInfo{Type: field.TypeInt64}},
	}
	targetMsg, err := builder.NewMessage("Target").Build()
	if err != nil {
		t.Fatal(err)
	}
	edgePbd := buildTestFieldDesc(t, "target", builder.FieldTypeMessage(builder.NewMessage("Target")), false)
	_, err = NewConverter(&entproto.FieldMappingDescriptor{
		IsEdgeField:       true,
		EntEdge:           &gen.Edge{Name: "target", Type: target, Unique: true},
		PbFieldDescriptor: edgePbd,
		ReferencedPbType:  targetMsg,
	}, "User")
	if err == nil || !strings.Contains(err.Error(), "no protobuf ID field") {
		t.Fatalf("expected missing protobuf ID field error, got %v", err)
	}
}

func TestNewConverter_SignedFixedTypesAndJSONWireShapes(t *testing.T) {
	scalarCases := []struct {
		ft      *builder.FieldType
		entType field.Type
		wantEnt string
		wantPb  string
	}{
		{builder.FieldTypeSInt32(), field.TypeInt, "int", "int32"},
		{builder.FieldTypeSFixed32(), field.TypeInt, "int", "int32"},
		{builder.FieldTypeSInt64(), field.TypeInt, "int", "int64"},
		{builder.FieldTypeSFixed64(), field.TypeInt, "int", "int64"},
		{builder.FieldTypeFixed32(), field.TypeUint, "uint", "uint32"},
		{builder.FieldTypeFixed64(), field.TypeUint, "uint", "uint64"},
	}
	for _, tc := range scalarCases {
		pbd := buildTestFieldDesc(t, "n", tc.ft, false)
		conv, err := NewConverter(&entproto.FieldMappingDescriptor{
			EntField:          &gen.Field{Name: "n", Type: &field.TypeInfo{Type: tc.entType}},
			PbFieldDescriptor: pbd,
		}, "User")
		if err != nil {
			t.Fatalf("scalar %v: %v", tc.ft.GetType(), err)
		}
		if conv.ToEntConversion != tc.wantEnt || conv.ToProtoConversion != tc.wantPb {
			t.Fatalf("scalar %v: got (%q, %q), want (%q, %q)", tc.ft.GetType(), conv.ToEntConversion, conv.ToProtoConversion, tc.wantEnt, tc.wantPb)
		}
	}

	validJSON := []struct {
		ident    string
		ft       *builder.FieldType
		repeated bool
	}{
		{"[]byte", builder.FieldTypeBytes(), false},
		{"[][]byte", builder.FieldTypeBytes(), true},
		{"[]bool", builder.FieldTypeBool(), true},
		{"[]string", builder.FieldTypeString(), true},
		{"[]int32", builder.FieldTypeSInt32(), true},
		{"[]int32", builder.FieldTypeSFixed32(), true},
		{"[]int64", builder.FieldTypeSInt64(), true},
		{"[]int64", builder.FieldTypeSFixed64(), true},
		{"[]uint32", builder.FieldTypeFixed32(), true},
		{"[]uint64", builder.FieldTypeFixed64(), true},
		{"[]float32", builder.FieldTypeFloat(), true},
		{"[]float64", builder.FieldTypeDouble(), true},
	}
	for _, tc := range validJSON {
		pbd := buildTestFieldDesc(t, "j", tc.ft, tc.repeated)
		conv, err := NewConverter(&entproto.FieldMappingDescriptor{
			EntField:          &gen.Field{Name: "j", Type: &field.TypeInfo{Type: field.TypeJSON, Ident: tc.ident}},
			PbFieldDescriptor: pbd,
		}, "User")
		if err != nil {
			t.Fatalf("valid JSON %s / %v (repeated=%v): %v", tc.ident, tc.ft.GetType(), tc.repeated, err)
		}
		if conv.ToProtoConversion != "" || conv.ToEntConversion != "" {
			t.Fatalf("valid JSON %s should use direct assignment, got %+v", tc.ident, conv)
		}
	}

	invalidJSON := []struct {
		ident    string
		ft       *builder.FieldType
		repeated bool
	}{
		{"[]int64", builder.FieldTypeInt32(), true},
		{"[]int64", builder.FieldTypeInt64(), false},
		{"[]string", builder.FieldTypeInt32(), true},
		{"[]byte", builder.FieldTypeString(), false},
		{"[]byte", builder.FieldTypeBytes(), true},
		{"map[string]any", builder.FieldTypeString(), true},
	}
	for _, tc := range invalidJSON {
		pbd := buildTestFieldDesc(t, "j", tc.ft, tc.repeated)
		if _, err := NewConverter(&entproto.FieldMappingDescriptor{
			EntField:          &gen.Field{Name: "j", Type: &field.TypeInfo{Type: field.TypeJSON, Ident: tc.ident}},
			PbFieldDescriptor: pbd,
		}, "User"); err == nil {
			t.Fatalf("mismatched JSON %s / %v (repeated=%v) expected error", tc.ident, tc.ft.GetType(), tc.repeated)
		}
	}
}
