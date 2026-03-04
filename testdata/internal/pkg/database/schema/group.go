package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

type Group struct {
	ent.Schema
}

func (Group) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entproto.Message(),
	}
}

func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Annotations(entproto.Field(2)),
		field.Bool("active").
			Default(true).
			Annotations(entproto.Field(3)),
		field.JSON("labels", []string{}).
			Annotations(entproto.Field(4)),
	}
}

func (Group) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("users", User.Type).
			Ref("groups").
			Annotations(entproto.Field(5)),
	}
}
