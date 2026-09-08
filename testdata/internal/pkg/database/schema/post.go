package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
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
		field.Int("id").
			Annotations(entproto.Field(1)),
		field.String("title").
			Annotations(entproto.Field(2)),
		field.Text("content").
			Annotations(entproto.Field(3)),
		field.Int("view_count").
			Annotations(entproto.Field(4)),
		field.Bool("published").
			Annotations(entproto.Field(5)),
		field.Enum("status").
			Values("pending", "in_progress", "done").
			Annotations(
				entproto.Field(6),
				entproto.Enum(map[string]int32{
					"pending":     1,
					"in_progress": 2,
					"done":        3,
				}),
			),
		field.Int64("likes").
			Annotations(entproto.Field(7)),
		field.Uint32("shares").
			Annotations(entproto.Field(8)),
		// Divergent-name enum: entproto derives the enum type as IPV4 (pascal)
		// but its proto values as IP_V4_* (snake). protoc-gen-go therefore emits
		// Post_IP_V4_*, not Post_IPV4_*. This guards the constant-name resolution
		// against re-deriving names from conventions (RF-202/203).
		field.Enum("ip_v4").
			Values("low", "high").
			Annotations(
				entproto.Field(10),
				entproto.Enum(map[string]int32{
					"low":  1,
					"high": 2,
				}),
			),
	}
}

func (Post) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).
			Ref("posts").
			Unique().
			Annotations(entproto.Field(9)),
	}
}
