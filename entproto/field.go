package entproto

import (
	"fmt"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema"
	"github.com/go-viper/mapstructure/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

const FieldAnnotation = "ProtoField"

type FieldOption func(*pbfield)

func Field(num int, options ...FieldOption) schema.Annotation {
	f := pbfield{Number: num}
	for _, apply := range options {
		apply(&f)
	}
	return f
}

type pbfield struct {
	Number   int
	Type     descriptorpb.FieldDescriptorProto_Type
	TypeName string
	// ProtoFile, when set, names the .proto file that defines TypeName for an
	// externally-defined message. It is filled in by MessageField so the
	// adapter can pick up the import without requiring a separate
	// RegisterCustomType call. It serialises along with the rest of the
	// annotation via mapstructure.
	ProtoFile string
}

func (f pbfield) Name() string {
	return FieldAnnotation
}

// Type overrides the default mapping between ent types and protobuf types.
// Example:
//
//	field.Uint8("custom_pb").
//		Annotations(
//			entproto.Field(2,
//				entproto.Type(descriptorpb.FieldDescriptorProto_TYPE_UINT64),
//			),
//		)
func Type(typ descriptorpb.FieldDescriptorProto_Type) FieldOption {
	return func(p *pbfield) {
		p.Type = typ
	}
}

// TypeName sets the pb descriptors type name, needed if the Type attribute is TYPE_ENUM or TYPE_MESSAGE.
func TypeName(n string) FieldOption {
	return func(p *pbfield) {
		p.TypeName = n
	}
}

// MessageField annotates an ent field that should be emitted as a protobuf
// message reference to an externally-defined type. It reads the fully-qualified
// type name and proto file path straight off the supplied generated Go message
// (no manual strings, no descriptorpb import needed); both are embedded in the
// annotation so the generator can wire up the import without a separate
// RegisterCustomType call.
//
// Typical usage on a JSON-backed column:
//
//	field.JSON("user", &sharedv1.User{}).
//		Annotations(entproto.MessageField(3, &sharedv1.User{}))
//
// The generator will emit `import "shared/v1/user.proto";` and reference the
// field as `shared.v1.User user = 3;`.
func MessageField(num int, msg proto.Message) schema.Annotation {
	if msg == nil {
		panic("entproto: MessageField called with nil message")
	}
	d := msg.ProtoReflect().Descriptor()
	if d == nil {
		panic(fmt.Sprintf("entproto: MessageField(%T) returned nil descriptor", msg))
	}
	parent := d.ParentFile()
	if parent == nil {
		panic(fmt.Sprintf("entproto: MessageField(%T) descriptor has no parent file", msg))
	}
	return pbfield{
		Number:    num,
		Type:      descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		TypeName:  string(d.FullName()),
		ProtoFile: parent.Path(),
	}
}

func extractFieldAnnotation(fld *gen.Field) (*pbfield, error) {
	annot, ok := fld.Annotations[FieldAnnotation]
	if !ok {
		return nil, fmt.Errorf("entproto: field %q does not have an entproto.Field annnoation", fld.Name)
	}

	var out pbfield
	err := mapstructure.Decode(annot, &out)
	if err != nil {
		return nil, fmt.Errorf("entproto: unable to decode entproto.Field annotation for field %q: %w",
			fld.Name, err)
	}

	return &out, nil
}

func extractEdgeAnnotation(edge *gen.Edge) (*pbfield, error) {
	annot, ok := edge.Annotations[FieldAnnotation]
	if !ok {
		return nil, fmt.Errorf("entproto: edge %q does not have an entproto.Field annotation", edge.Name)
	}

	var out pbfield
	err := mapstructure.Decode(annot, &out)
	if err != nil {
		return nil, fmt.Errorf("entproto: unable to decode entproto.Field annotation for field %q: %w",
			edge.Name, err)
	}

	return &out, nil
}
