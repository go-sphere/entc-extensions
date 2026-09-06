package entproto

import (
	"testing"

	"github.com/jhump/protoreflect/desc/builder"
)

func TestCamelCase_DigitsAndUnderscores(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "_my_field_name_2", want: "XMyFieldName_2"},
		{in: "user_name", want: "UserName"},
		{in: "field9name", want: "Field9Name"},
		{in: "abc123xyz", want: "Abc123Xyz"},
	}

	for _, tc := range tests {
		if got := camelCase(tc.in); got != tc.want {
			t.Fatalf("camelCase(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFieldMappingDescriptor_OptionalPointerRepresentation(t *testing.T) {
	messageBuilder := builder.NewMessage("Fixture").
		AddField(builder.NewField("name", builder.FieldTypeString()).SetNumber(2).SetProto3Optional(true)).
		AddField(builder.NewField("data", builder.FieldTypeBytes()).SetNumber(3).SetProto3Optional(true))
	file, err := builder.NewFile("fixture.proto").SetProto3(true).AddMessage(messageBuilder).Build()
	if err != nil {
		t.Fatalf("build descriptor: %v", err)
	}
	message := file.FindMessage("Fixture")
	testCases := []struct {
		name string
		want bool
	}{
		{name: "name", want: true},
		{name: "data", want: false},
	}
	for _, testCase := range testCases {
		name := testCase.name
		field := message.FindFieldByName(name)
		descriptor := &FieldMappingDescriptor{PbFieldDescriptor: field}
		if got := descriptor.IsProto3OptionalPointer(); got != testCase.want {
			t.Fatalf("IsProto3OptionalPointer(%s) = %v, want %v", name, got, testCase.want)
		}
	}
}

func TestFieldMappingDescriptor_PbFieldNameUsesProtocGoNaming(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "field9name", want: "Field9Name"},
		{in: "abc123xyz", want: "Abc123Xyz"},
		{in: "_my_field_name_2", want: "XMyFieldName_2"},
	}
	for _, tc := range tests {
		message, err := builder.NewMessage("Fixture").
			AddField(builder.NewField(tc.in, builder.FieldTypeString()).SetNumber(2)).
			Build()
		if err != nil {
			t.Fatalf("build descriptor for %q: %v", tc.in, err)
		}
		field := message.FindFieldByName(tc.in)
		descriptor := &FieldMappingDescriptor{PbFieldDescriptor: field}
		if got := descriptor.PbFieldName(); got != tc.want {
			t.Fatalf("PbFieldName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
