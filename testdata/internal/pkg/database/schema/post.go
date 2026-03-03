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
	}
}
