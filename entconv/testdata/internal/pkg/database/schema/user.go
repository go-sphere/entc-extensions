package schema

import (
	"entgo.io/contrib/entproto"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
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
		field.String("name").
			Annotations(entproto.Field(2)),
		field.Int("age").
			Annotations(entproto.Field(3)),
		field.Bool("active").
			Annotations(entproto.Field(4)),
		field.Float("score").
			Annotations(entproto.Field(5)),
		field.Time("birthday").
			Annotations(entproto.Field(6)),
		field.Bytes("avatar").
			Annotations(entproto.Field(7)),
		field.String("status").
			Annotations(entproto.Field(8)),
		field.String("email").
			Annotations(entproto.Field(9)),
		field.Int64("balance").
			Annotations(entproto.Field(10)),
		field.Uint("role").
			Annotations(entproto.Field(11)),
	}
}
