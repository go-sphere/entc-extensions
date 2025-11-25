package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Example struct {
	ent.Schema
}

type EdgeItem struct {
	ent.Schema
}

func (Example) Fields() []ent.Field {
	return []ent.Field{
		field.Int("int_value"),
		field.Ints("ints_value"),
		field.Int8("int8_value"),
		field.Int16("int16_value"),
		field.Int32("int32_value"),
		field.Int64("int64_value"),
		field.Uint("uint_value"),
		field.Uint8("uint8_value"),
		field.Uint16("uint16_value"),
		field.Uint32("uint32_value"),
		field.Uint64("uint64_value"),
		field.Float32("float32_value"),
		field.Float("float_value"),
		field.Floats("floats_value"),
		field.Bool("bool_value"),
		field.String("string_value"),
		field.Strings("strings_value"),
		field.Bytes("bytes_value"),
		field.JSON("json_value", map[string]interface{}{}),
		field.Enum("enum_value").Values("A", "B", "C"),
		field.Text("text_value"),
		field.Any("any_value"),
	}
}

func (EdgeItem) Fields() []ent.Field {
	return []ent.Field{
		field.Int("int_value").Optional(),
		field.Ints("ints_value").Optional(),
		field.Int8("int8_value").Optional(),
		field.Int16("int16_value").Optional(),
		field.Int32("int32_value").Optional(),
		field.Int64("int64_value").Optional(),
		field.Uint("uint_value").Optional(),
		field.Uint8("uint8_value").Optional(),
		field.Uint16("uint16_value").Optional(),
		field.Uint32("uint32_value").Optional(),
		field.Uint64("uint64_value").Optional(),
		field.Float32("float32_value").Optional(),
		field.Float("float_value").Optional(),
		field.Floats("floats_value").Optional(),
		field.Bool("bool_value").Optional(),
		field.String("string_value").Optional(),
		field.Strings("strings_value").Optional(),
		field.Bytes("bytes_value").Optional(),
		field.JSON("json_value", map[string]interface{}{}).Optional(),
		field.Enum("enum_value").Values("A", "B", "C").Optional(),
		field.Text("text_value").Optional(),
		field.Any("any_value").Optional(),
	}
}

func (Example) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("edge_item", EdgeItem.Type),
	}
}
