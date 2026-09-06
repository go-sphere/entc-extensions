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
		field.Int8("rank").
			Annotations(entproto.Field(12)),
		field.Uint64("quota").
			Annotations(entproto.Field(13)),
		field.JSON("tags", []string{}).
			Annotations(entproto.Field(14)),
		field.JSON("points", []int64{}).
			Annotations(entproto.Field(15)),
		field.String("optional_note").
			Optional().
			Nillable().
			Annotations(entproto.Field(18)),
		field.Bytes("optional_blob").
			Optional().
			Nillable().
			Annotations(entproto.Field(19)),
		field.Int("optional_count").
			Optional().
			Nillable().
			Annotations(entproto.Field(20)),
		field.Time("optional_birthday").
			Optional().
			Nillable().
			Annotations(entproto.Field(21)),
		field.Enum("optional_level").
			Values("low", "high").
			Optional().
			Nillable().
			Annotations(
				entproto.Field(22),
				entproto.Enum(map[string]int32{
					"low":  1,
					"high": 2,
				}),
			),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("posts", Post.Type).
			Annotations(entproto.Field(16)),
		edge.To("groups", Group.Type).
			Annotations(entproto.Field(17)),
	}
}
