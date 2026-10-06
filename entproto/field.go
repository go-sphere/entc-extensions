package entproto

import (
	"fmt"
	"strings"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema"
	"github.com/go-viper/mapstructure/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FieldAnnotation is the annotation name under which [Field] and
// [MessageField] are stored in ent field and edge annotations.
const FieldAnnotation = "ProtoField"

// FieldOption configures the annotation returned by [Field].
type FieldOption func(*pbfield)

// Field annotates an ent field or edge with its protobuf field number num.
// Numbers must be unique within the message and must fit in int32; number 1 is
// reserved for the ID field and is rejected on other fields and edges. The
// default ent ID gets number 1 implicitly; an id field declared in the schema's
// Fields needs Field(1) (or [WithAutoFill]).
//
// Without options, the protobuf type follows the ent type: bool, string, and
// numeric types map to the matching scalar (int, int64 -> int64; int8-int32 ->
// int32; uint, uint8-uint32 -> uint32; uint64 -> uint64; float32 -> float;
// float64 -> double), time.Time -> int64, UUID and bytes -> bytes, enum -> a
// nested enum configured by [Enum], and JSON []string, []int32, []int64,
// []uint32, or []uint64 -> the matching repeated scalar. Other JSON and Other
// fields fail unless [Type] or [MessageField] supplies the type. Nillable
// fields that are neither repeated nor messages become proto3 optional fields. An edge becomes a
// message reference to the target schema, repeated unless the edge is unique.
func Field(num int, options ...FieldOption) schema.Annotation {
	f := pbfield{Number: num}
	for _, apply := range options {
		apply(&f)
	}
	return f
}

type pbfield struct {
	Number       int
	Type         descriptorpb.FieldDescriptorProto_Type
	TypeName     string
	ProtoPackage string
	MessagePath  string
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

// Type overrides the protobuf type that [Field] would derive from the ent type.
// TYPE_ENUM and TYPE_MESSAGE also require [TypeName]. The override only
// affects the .proto output; the ent field's Go type must still be compatible
// with the Go type protoc-gen-go generates. Example:
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

// TypeName sets the fully-qualified protobuf type name (for example
// "google.protobuf.Timestamp"; a leading dot is optional) used with [Type]
// TYPE_ENUM or TYPE_MESSAGE. TypeName has no effect without [Type]. A message
// defined outside the generated files must also be registered with
// [RegisterCustomType]; [MessageField] does both in one step.
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
// RegisterCustomType call. It panics if msg is nil or has no file descriptor.
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
		Number:       num,
		Type:         descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		TypeName:     string(d.FullName()),
		ProtoFile:    parent.Path(),
		ProtoPackage: string(parent.Package()),
		MessagePath:  strings.TrimPrefix(string(d.FullName()), string(parent.Package())+"."),
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
