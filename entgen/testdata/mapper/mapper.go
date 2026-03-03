package mapper

import (
	"github.com/go-sphere/entc-extensions/entgen/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/entgen/testdata/ent/edgeitem"
	"github.com/go-sphere/entc-extensions/entgen/testdata/ent/example"
)

func ExampleEnumMap(v entpb.Example_EnumValue) example.EnumValue {
	switch v {
	case entpb.Example_A:
		return example.EnumValueA
	case entpb.Example_B:
		return example.EnumValueB
	case entpb.Example_C:
		return example.EnumValueC
	default:
		return ""
	}
}

func EdgeItemEnumUnmap(v entpb.EdgeItem_EnumValue) edgeitem.EnumValue {
	switch v {
	case entpb.EdgeItem_A:
		return edgeitem.EnumValueA
	case entpb.EdgeItem_B:
		return edgeitem.EnumValueB
	case entpb.EdgeItem_C:
		return edgeitem.EnumValueC
	default:
		return ""
	}
}
