package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
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
		field.String("name").
			Annotations(entproto.Field(2)),
		field.Int("age").
			Annotations(entproto.Field(3)),
		field.Bool("active").
			Annotations(entproto.Field(4)),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("posts", Post.Type).
			Annotations(entproto.Field(5)),
	}
}
