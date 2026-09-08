package enumschema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/go-sphere/entc-extensions/entproto"
)

// User exercises an enum field whose name contains an underscore and a digit.
// entproto names the enum type IPV4 (pascal) but its values IP_V4_* (snake),
// which diverges from the enum type prefix protoc-gen-go would strip.
type User struct {
	ent.Schema
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{entproto.Message()}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("ip_v4").
			Values("low", "high").
			Annotations(
				entproto.Field(2),
				entproto.Enum(map[string]int32{"low": 1, "high": 2}),
			),
	}
}
