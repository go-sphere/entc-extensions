package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

type User struct {
	ent.Schema
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entproto.Message(),
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Annotations(entproto.Field(1)),
		field.String("name").
			Annotations(entproto.Field(2)),
	}
}
