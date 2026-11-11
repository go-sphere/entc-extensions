package entproto

import (
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"google.golang.org/protobuf/types/descriptorpb"
)

var typeMap = map[field.Type]typeConfig{
	field.TypeBool:  {pbType: descriptorpb.FieldDescriptorProto_TYPE_BOOL},
	field.TypeTime:  {pbType: descriptorpb.FieldDescriptorProto_TYPE_INT64},
	field.TypeOther: {unsupported: true},
	field.TypeUUID:  {pbType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
	field.TypeBytes: {pbType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
	field.TypeEnum: {pbType: descriptorpb.FieldDescriptorProto_TYPE_ENUM, namer: func(fld *gen.Field) string {
		return pascal(fld.Name)
	}},
	field.TypeString:  {pbType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
	field.TypeInt:     {pbType: descriptorpb.FieldDescriptorProto_TYPE_INT64},
	field.TypeInt8:    {pbType: descriptorpb.FieldDescriptorProto_TYPE_INT32},
	field.TypeInt16:   {pbType: descriptorpb.FieldDescriptorProto_TYPE_INT32},
	field.TypeInt32:   {pbType: descriptorpb.FieldDescriptorProto_TYPE_INT32},
	field.TypeInt64:   {pbType: descriptorpb.FieldDescriptorProto_TYPE_INT64},
	field.TypeUint:    {pbType: descriptorpb.FieldDescriptorProto_TYPE_UINT64},
	field.TypeUint8:   {pbType: descriptorpb.FieldDescriptorProto_TYPE_UINT32},
	field.TypeUint16:  {pbType: descriptorpb.FieldDescriptorProto_TYPE_UINT32},
	field.TypeUint32:  {pbType: descriptorpb.FieldDescriptorProto_TYPE_UINT32},
	field.TypeUint64:  {pbType: descriptorpb.FieldDescriptorProto_TYPE_UINT64},
	field.TypeFloat32: {pbType: descriptorpb.FieldDescriptorProto_TYPE_FLOAT},
	field.TypeFloat64: {pbType: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE},
}

// repeatedJSONTypes maps the Go type of a JSON field to the element type of
// the repeated protobuf field it becomes.
var repeatedJSONTypes = map[string]descriptorpb.FieldDescriptorProto_Type{
	"[]string": descriptorpb.FieldDescriptorProto_TYPE_STRING,
	"[]int32":  descriptorpb.FieldDescriptorProto_TYPE_INT32,
	"[]int64":  descriptorpb.FieldDescriptorProto_TYPE_INT64,
	"[]uint32": descriptorpb.FieldDescriptorProto_TYPE_UINT32,
	"[]uint64": descriptorpb.FieldDescriptorProto_TYPE_UINT64,
}

type typeConfig struct {
	unsupported bool
	pbType      descriptorpb.FieldDescriptorProto_Type
	namer       func(fld *gen.Field) string
}
