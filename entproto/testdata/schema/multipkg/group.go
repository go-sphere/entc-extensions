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
		entproto.Message(entproto.PackageName("acme.group.v1")),
	}
}

func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Annotations(entproto.Field(2)),
	}
}

func (Group) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("owner", User.Type).
			Unique().
			Annotations(entproto.Field(3)),
		edge.To("members", User.Type).
			Annotations(entproto.Field(4)),
	}
}
