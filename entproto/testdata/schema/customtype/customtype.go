package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Admin struct {
	ent.Schema
}

func (Admin) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entproto.Message(),
	}
}

func (Admin) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Annotations(entproto.Field(2)),
		field.JSON("ts", &timestamppb.Timestamp{}).
			Annotations(entproto.MessageField(3, &timestamppb.Timestamp{})),
	}
}
