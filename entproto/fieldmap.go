package entproto

import (
	"fmt"
	"slices"
	"strings"

	"entgo.io/ent/entc/gen"
	"github.com/jhump/protoreflect/desc" //nolint:staticcheck // Adapter APIs still expose jhump v1 descriptors.
	"google.golang.org/protobuf/types/descriptorpb"
)

// FieldMap returns the mapping between the protobuf fields of the message for
// schemaName and the ent fields and edges they came from. It returns the same
// errors as [Adapter.GetMessageDescriptor], or an error if schemaName is not in
// the graph.
func (a *Adapter) FieldMap(schemaName string) (FieldMap, error) {
	bt, ok := a.nodeByName[schemaName]
	if !ok {
		return nil, fmt.Errorf("entproto: could not find schema %q in graph", schemaName)
	}
	md, err := a.GetMessageDescriptor(schemaName)
	if err != nil {
		return nil, err
	}
	return a.mapFields(bt, md)
}

// FieldMap maps protobuf field names (equal to the ent field or edge names) to
// their [FieldMappingDescriptor]. Use its methods for sorted, filtered views.
type FieldMap map[string]*FieldMappingDescriptor

// Fields returns the descriptors of all non-edge fields, including the ID,
// sorted by [FieldMappingDescriptor.PbStructField].
func (m FieldMap) Fields() []*FieldMappingDescriptor {
	return m.sorted(func(f *FieldMappingDescriptor) bool { return !f.IsEdgeField })
}

// ID returns the descriptor of the ID field, or nil if the message has none
// (for example when the ID is annotated with [Skip]).
func (m FieldMap) ID() *FieldMappingDescriptor {
	for _, f := range m {
		if f.IsIDField {
			return f
		}
	}
	return nil
}

// Edges returns the descriptors of all edge fields, sorted by
// [FieldMappingDescriptor.PbStructField].
func (m FieldMap) Edges() []*FieldMappingDescriptor {
	return m.sorted(func(f *FieldMappingDescriptor) bool { return f.IsEdgeField })
}

// Enums returns the descriptors of all enum-typed fields, sorted by
// [FieldMappingDescriptor.PbStructField].
func (m FieldMap) Enums() []*FieldMappingDescriptor {
	return m.sorted(func(f *FieldMappingDescriptor) bool { return f.IsEnumField })
}

func (m FieldMap) sorted(keep func(*FieldMappingDescriptor) bool) []*FieldMappingDescriptor {
	var out []*FieldMappingDescriptor
	for _, f := range m {
		if keep(f) {
			out = append(out, f)
		}
	}
	slices.SortFunc(out, func(a, b *FieldMappingDescriptor) int {
		return strings.Compare(a.PbStructField(), b.PbStructField())
	})
	return out
}

// FieldMappingDescriptor describes how one protobuf field maps to an ent field
// or edge. Exactly one of EntField and EntEdge is set, as indicated by
// IsEdgeField; ReferencedPbType is the target message for edges and nil
// otherwise.
type FieldMappingDescriptor struct {
	EntField          *gen.Field
	EntEdge           *gen.Edge
	PbFieldDescriptor *desc.FieldDescriptor
	IsEdgeField       bool
	IsIDField         bool
	IsEnumField       bool
	ReferencedPbType  *desc.MessageDescriptor
}

// PbStructField returns the Go struct field name protoc-gen-go generates for
// the protobuf field, for example "created_at" -> "CreatedAt".
func (d *FieldMappingDescriptor) PbStructField() string {
	return camelCase(d.PbFieldDescriptor.GetName())
}

// PbFieldName returns the same Go struct field name as
// [FieldMappingDescriptor.PbStructField].
func (d *FieldMappingDescriptor) PbFieldName() string {
	return d.PbStructField()
}

// IsProto3Optional reports whether the protobuf field is a proto3 optional
// field, which entproto emits for nillable ent fields.
func (d *FieldMappingDescriptor) IsProto3Optional() bool {
	return d.PbFieldDescriptor.AsFieldDescriptorProto().GetProto3Optional()
}

// PbEnumValueConstName returns the Go constant name protoc-gen-go emits for the
// protobuf enum value that corresponds to the ent enum value `value`. The
// protobuf value descriptor is resolved through the entproto.Enum annotation's
// configured number, so the name is read from the real descriptor instead of
// being re-derived from naming conventions (which diverge for names such as
// ip_v4 or api_v2). protoc-gen-go names a value nested in message M as
// "M_<proto value name>", and enum value names are not camel-cased.
func (d *FieldMappingDescriptor) PbEnumValueConstName(value string) (string, error) {
	if d.EntField == nil {
		return "", fmt.Errorf("entproto: enum field %q has no ent field mapping", d.PbFieldDescriptor.GetName())
	}
	enum := d.PbFieldDescriptor.GetEnumType()
	if enum == nil {
		return "", fmt.Errorf("entproto: field %q is not an enum", d.PbFieldDescriptor.GetName())
	}
	number, ok := EnumValueNumber(d.EntField, value)
	if !ok {
		return "", fmt.Errorf("entproto: ent enum value %q on field %q has no protobuf number annotation", value, d.EntField.Name)
	}
	vd := enum.FindValueByNumber(number)
	if vd == nil {
		return "", fmt.Errorf("entproto: proto enum %s has no value numbered %d (ent value %q)", enum.GetName(), number, value)
	}
	prefix := enum.GetName()
	if md, ok := enum.GetParent().(*desc.MessageDescriptor); ok {
		prefix = md.GetName()
	}
	return prefix + "_" + vd.GetName(), nil
}

// IsProto3OptionalPointer reports whether protoc-gen-go represents this
// proto3 optional field with a pointer. Optional bytes retain their native
// []byte representation and use nil for absence.
func (d *FieldMappingDescriptor) IsProto3OptionalPointer() bool {
	return d.IsProto3Optional() && d.PbFieldDescriptor.GetType() != descriptorpb.FieldDescriptorProto_TYPE_BYTES
}

// EdgeIDPbStructField returns the Go struct field name of the ID field of the
// entity this edge refers to. It panics if the descriptor is not an edge.
func (d *FieldMappingDescriptor) EdgeIDPbStructField() string {
	return camelCase(d.EntEdge.Type.ID.Name)
}

// EdgeIDPbStructFieldDesc returns the protobuf field descriptor for the ID
// field of the message this edge refers to, or nil if that message has no
// such field. It panics if the descriptor is not an edge.
func (d *FieldMappingDescriptor) EdgeIDPbStructFieldDesc() *desc.FieldDescriptor {
	return d.ReferencedPbType.FindFieldByName(d.EntEdge.Type.ID.Name)
}

func (a *Adapter) mapFields(entType *gen.Type, pbType *desc.MessageDescriptor) (FieldMap, error) {
	fieldByName := make(map[string]*gen.Field, len(entType.Fields)+1)
	fieldByName[entType.ID.Name] = entType.ID
	for _, fld := range entType.Fields {
		fieldByName[fld.Name] = fld
	}
	edgeByName := make(map[string]*gen.Edge, len(entType.Edges))
	for _, edg := range entType.Edges {
		edgeByName[edg.Name] = edg
	}

	m := make(map[string]*FieldMappingDescriptor, len(pbType.GetFields()))
	for _, fld := range pbType.GetFields() {
		fd := &FieldMappingDescriptor{
			PbFieldDescriptor: fld,
			IsIDField:         fld.GetName() == entType.ID.Name,
			IsEnumField:       fld.GetEnumType() != nil,
		}
		edg, isEdge := edgeByName[fld.GetName()]
		if isEdge {
			fd.IsEdgeField = true
			fd.EntEdge = edg
			referenced, err := a.GetMessageDescriptor(edg.Type.Name)
			if err != nil {
				return nil, err
			}
			fd.ReferencedPbType = referenced
		} else {
			enf, ok := fieldByName[fld.GetName()]
			if !ok {
				return nil, fmt.Errorf("entproto: could not find field %q in %q", fld.GetName(), entType.Name)
			}
			fd.EntField = enf
		}
		m[fld.GetName()] = fd
	}
	return m, nil
}

// Is c an ASCII lower-case letter?
func isASCIILower(c byte) bool {
	return 'a' <= c && c <= 'z'
}

// Is c an ASCII digit?
func isASCIIDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

// camelCase was copied from https://github.com/golang/protobuf/blob/v1.5.2/protoc-gen-go/generator/generator.go#L2648
// camelCase returns the CamelCased name.
// If there is an interior underscore followed by a lower case letter,
// drop the underscore and convert the letter to upper case.
// There is a remote possibility of this rewrite causing a name collision,
// but it's so remote we're prepared to pretend it's nonexistent - since the
// C++ generator lowercases names, it's extremely unlikely to have two fields
// with different capitalizations.
// In short, _my_field_name_2 becomes XMyFieldName_2.
func camelCase(s string) string {
	if s == "" {
		return ""
	}
	t := make([]byte, 0, 32)
	i := 0
	if s[0] == '_' {
		// Need a capital letter; drop the '_'.
		t = append(t, 'X')
		i++
	}
	// Invariant: if the next letter is lower case, it must be converted
	// to upper case.
	// That is, we process a word at a time, where words are marked by _ or
	// upper case letter. Digits are treated as words.
	for ; i < len(s); i++ {
		c := s[i]
		if c == '_' && i+1 < len(s) && isASCIILower(s[i+1]) {
			continue // Skip the underscore in s.
		}
		if isASCIIDigit(c) {
			t = append(t, c)
			continue
		}
		// Assume we have a letter now - if not, it's a bogus identifier.
		// The next word is a sequence of characters that must start upper case.
		if isASCIILower(c) {
			c ^= ' ' // Make it a capital letter.
		}
		t = append(t, c) // Guaranteed not lower case.
		// Accept lower case sequence that follows.
		for i+1 < len(s) && isASCIILower(s[i+1]) {
			i++
			t = append(t, s[i])
		}
	}
	return string(t)
}
