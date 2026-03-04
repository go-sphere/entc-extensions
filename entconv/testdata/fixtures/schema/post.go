package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

type Post struct {
	ent.Schema
}

func (Post) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entproto.Message(),
	}
}

func (Post) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			Annotations(entproto.Field(2)),
	}
}
