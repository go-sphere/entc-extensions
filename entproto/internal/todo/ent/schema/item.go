package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Item struct {
	ent.Schema
}

func (Item) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Int64("quantity"),
		field.Bool("available"),
	}
}
