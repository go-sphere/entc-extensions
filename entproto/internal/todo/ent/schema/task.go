package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/go-sphere/entc-extensions/entproto"
)

type Task struct {
	ent.Schema
}

func (Task) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entproto.Message(),
	}
}

func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("status").
			Values("pending", "in_progress", "done").
			Default("pending").
			Annotations(
				entproto.Field(3),
				entproto.Enum(map[string]int32{
					"pending":     0,
					"in_progress": 1,
					"done":        2,
				}),
			),
	}
}
