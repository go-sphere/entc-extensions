package autofillschema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

// Item has no entproto annotations and requires WithAutoFill.
type Item struct {
	ent.Schema
}

func (Item) Fields() []ent.Field {
	return []ent.Field{
		field.String("title"),
	}
}

// Hidden opts out of proto generation even with WithAutoFill.
type Hidden struct {
	ent.Schema
}

func (Hidden) Annotations() []schema.Annotation {
	return []schema.Annotation{entproto.SkipGen()}
}

func (Hidden) Fields() []ent.Field {
	return []ent.Field{
		field.String("secret"),
	}
}
