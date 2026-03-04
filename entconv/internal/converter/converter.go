package converter

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
	"github.com/jhump/protoreflect/desc"
	dpb "google.golang.org/protobuf/types/descriptorpb"
)

var (
	binaryMarshallerUnmarshalerType = reflect.TypeFor[BinaryMarshallerUnmarshaler]()
)

// BinaryMarshallerUnmarshaler is an interface for types that can marshal/unmarshal themselves to/from binary format.
type BinaryMarshallerUnmarshaler interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// Converter holds conversion information for a single field.
type Converter struct {
	ToEntConversion              string
	ToEntConversionArg           string // Additional argument for conversion (e.g., nanoseconds for time.Unix)
	ToEntScannerConversion       string
	ToEntConstructor             string
	ToEntMarshallerConstructor   string
	ToEntScannerConstructor      string
	ToEntModifier                string
	ToProtoConversion            string
	ToProtoConversionModifier    string // Postfix to apply (e.g., .Unix() for time fields)
	ToProtoConstructor           string
	ToProtoMarshallerConstructor string
	ToProtoValuer                string
}

// NewConverter creates a Converter for the given field mapping and type name.
func NewConverter(fld *entproto.FieldMappingDescriptor, typeName string) (*Converter, error) {
	out := &Converter{}
	pbd := fld.PbFieldDescriptor
	switch pbd.GetType() {
	case dpb.FieldDescriptorProto_TYPE_BOOL, dpb.FieldDescriptorProto_TYPE_STRING,
		dpb.FieldDescriptorProto_TYPE_BYTES, dpb.FieldDescriptorProto_TYPE_INT32,
		dpb.FieldDescriptorProto_TYPE_INT64, dpb.FieldDescriptorProto_TYPE_UINT32,
		dpb.FieldDescriptorProto_TYPE_UINT64, dpb.FieldDescriptorProto_TYPE_FLOAT,
		dpb.FieldDescriptorProto_TYPE_DOUBLE:
		if err := basicTypeConversion(fld.PbFieldDescriptor, fld.EntField, out); err != nil {
			return nil, err
		}
	case dpb.FieldDescriptorProto_TYPE_ENUM:
		enumName := fld.PbFieldDescriptor.GetEnumType().GetName()
		method := fmt.Sprintf("ToProto%s_%s", typeName, enumName)
		out.ToProtoConstructor = method
	case dpb.FieldDescriptorProto_TYPE_MESSAGE:
		switch {
		case fld.IsEdgeField:
			if err := basicTypeConversion(fld.EdgeIDPbStructFieldDesc(), fld.EntEdge.Type.ID, out); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("entproto: no mapping for pb message type %q", pbd.GetMessageType().GetFullyQualifiedName())
		}
	default:
		return nil, fmt.Errorf("entproto: no mapping for pb field type %q", pbd.GetType())
	}
	efld := fld.EntField
	if fld.IsEdgeField {
		efld = fld.EntEdge.Type.ID
	}

	switch {
	case fld.IsIDField || (fld.EntField != nil && strings.ToLower(fld.EntField.Name) == "id"):
		// ID field - use the ent field type directly
		// For ID fields, Type.Ident may be empty, so use Type.Type.String() instead
		// e.g., "int", "int64", "uint", "string" for uuid
		idType := ""
		if efld.Type.Ident != "" {
			idType = efld.Type.Ident
		} else {
			idType = efld.Type.Type.String()
		}
		out.ToEntConversion = idType
	case implements(efld.Type.RType, binaryMarshallerUnmarshalerType) && efld.HasGoType():
		// Ident returned from ent already has the packagename prefixed. Strip it since `g.QualifiedGoIdent`
		// adds it back.
		out.ToEntMarshallerConstructor = constructorName(efld.Type.Ident)
	case efld.Type.ValueScanner():
		switch {
		case efld.HasGoType():
			// Ident returned from ent already has the packagename prefixed. Strip it since `g.QualifiedGoIdent`
			// adds it back.
			out.ToEntScannerConstructor = constructorName(efld.Type.Ident)
		case efld.IsBool():
			out.ToEntScannerConversion = "bool"
		case efld.IsBytes():
			out.ToEntScannerConversion = "[]byte"
		case efld.IsString():
			out.ToEntScannerConversion = "string"
		}
	case efld.IsBool(), efld.IsBytes(), efld.IsString():
	case efld.Type.Numeric():
		out.ToEntConversion = efld.Type.String()
	case efld.IsTime():
		out.ToProtoConversion = "int64"
		out.ToProtoConversionModifier = ".Unix()"
		out.ToEntConversion = "time.Unix"
		out.ToEntConversionArg = "0"
	case efld.IsEnum():
		enumName := fld.PbFieldDescriptor.GetEnumType().GetName()
		method := fmt.Sprintf("ToEnt%s_%s", typeName, enumName)
		out.ToEntConstructor = method
	case efld.IsJSON():
		switch efld.Type.Ident {
		case "[]string":
		case "[]int32", "[]int64", "[]uint32", "[]uint64":
			out.ToProtoConversion = ""
		default:
			return nil, fmt.Errorf("entproto: no mapping to ent field type %q", efld.Type.ConstName())
		}
	default:
		return nil, fmt.Errorf("entproto: no mapping to ent field type %q", efld.Type.ConstName())
	}
	return out, nil
}

// Supported value scanner types (https://golang.org/pkg/database/sql/driver/#Value): [int64, float64, bool, []byte, string, time.Time]
func basicTypeConversion(md *desc.FieldDescriptor, entField *gen.Field, conv *Converter) error {
	switch md.GetType() {
	case dpb.FieldDescriptorProto_TYPE_BOOL:
		if entField.Type.Valuer() {
			conv.ToProtoValuer = "bool"
		}
	case dpb.FieldDescriptorProto_TYPE_STRING:
		if entField.Type.Valuer() {
			conv.ToProtoValuer = "string"
		}
	case dpb.FieldDescriptorProto_TYPE_BYTES:
		if implements(entField.Type.RType, binaryMarshallerUnmarshalerType) {
			// Ident returned from ent already has the packagename prefixed. Strip it since `g.QualifiedGoIdent`
			// adds it back.
			conv.ToProtoMarshallerConstructor = constructorName(entField.Type.Ident)
		} else if entField.Type.Valuer() {
			conv.ToProtoValuer = "[]byte"
		}
	case dpb.FieldDescriptorProto_TYPE_INT32:
		if entField.Type.String() != "int32" {
			conv.ToProtoConversion = "int32"
		}
	case dpb.FieldDescriptorProto_TYPE_INT64:
		if entField.Type.Valuer() {
			conv.ToProtoValuer = "int64"
		} else if entField.Type.String() != "int64" {
			conv.ToProtoConversion = "int64"
		}
	case dpb.FieldDescriptorProto_TYPE_UINT32:
		if entField.Type.String() != "uint32" {
			conv.ToProtoConversion = "uint32"
		}
	case dpb.FieldDescriptorProto_TYPE_UINT64:
		if entField.Type.String() != "uint64" {
			conv.ToProtoConversion = "uint64"
		}
	case dpb.FieldDescriptorProto_TYPE_FLOAT:
		if entField.Type.String() != "float32" {
			conv.ToProtoConversion = "float32"
		}
	case dpb.FieldDescriptorProto_TYPE_DOUBLE:
		if entField.Type.Valuer() {
			conv.ToProtoConversion = "float64"
		}
	}
	return nil
}

func implements(r *field.RType, typ reflect.Type) bool {
	if r == nil {
		return false
	}
	n := typ.NumMethod()
	for i := range n {
		m0 := typ.Method(i)
		m1, ok := r.Methods[m0.Name]
		if !ok || len(m1.In) != m0.Type.NumIn() || len(m1.Out) != m0.Type.NumOut() {
			return false
		}
		in := m0.Type.NumIn()
		for j := range in {
			if !m1.In[j].TypeEqual(m0.Type.In(j)) {
				return false
			}
		}
		out := m0.Type.NumOut()
		for j := range out {
			if !m1.Out[j].TypeEqual(m0.Type.Out(j)) {
				return false
			}
		}
	}
	return true
}

func constructorName(ident string) string {
	if idx := strings.LastIndex(ident, "."); idx >= 0 && idx+1 < len(ident) {
		return ident[idx+1:]
	}
	return ident
}
