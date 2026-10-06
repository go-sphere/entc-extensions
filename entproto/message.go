package entproto

import (
	"fmt"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema"
	"github.com/go-viper/mapstructure/v2"
)

// MessageAnnotation is the annotation name under which [Message] and [SkipGen]
// are stored in ent schema annotations (gen.Type.Annotations).
const MessageAnnotation = "ProtoMessage"

// MessageOption configures the annotation returned by [Message].
type MessageOption func(msg *message)

// Message annotates an ent schema so a protobuf message named after the schema
// is generated for it. Return it from the schema's Annotations method. The
// message is placed in proto package [DefaultProtoPackageName] ("entpb")
// unless [PackageName] is given.
//
// Without [WithAutoFill], schemas lacking this annotation are not generated,
// and every non-ID field and edge of an annotated schema needs a [Field] or
// [Skip] annotation.
func Message(opts ...MessageOption) schema.Annotation {
	m := message{
		Generate: true,
		Package:  "entpb",
	}
	for _, apply := range opts {
		apply(&m)
	}
	return m
}

// SkipGen annotates an ent schema so no protobuf message is generated for it,
// even with [WithAutoFill]. Use it to override a [Message] annotation inherited
// from a mixin. Edges from generated schemas to a SkipGen schema must be
// annotated with [Skip], otherwise generation fails.
func SkipGen() schema.Annotation {
	return message{
		Generate: false,
	}
}

// PackageName sets the protobuf package of the message, for example
// "acme.user.v1", which is written to acme/user/v1/v1.proto. An empty pkg
// falls back to [DefaultProtoPackageName]. Messages that share a package share
// one file; references across packages add the matching import.
func PackageName(pkg string) MessageOption {
	return func(msg *message) {
		msg.Package = pkg
	}
}

type message struct {
	Generate bool
	Package  string
}

func (m message) Name() string {
	return MessageAnnotation
}

func (message) Merge(other schema.Annotation) schema.Annotation {
	return other
}

func extractMessageAnnotation(sch *gen.Type) (*message, error) {
	annot, ok := sch.Annotations[MessageAnnotation]
	if !ok {
		return nil, fmt.Errorf("entproto: schema %q does not have an entproto.Message annotation", sch.Name)
	}

	var out message
	err := mapstructure.Decode(annot, &out)
	if err != nil {
		return nil, fmt.Errorf("entproto: unable to decode entproto.Message annotation for schema %q: %w",
			sch.Name, err)
	}

	return &out, nil
}
