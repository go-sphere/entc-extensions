package entproto

import "testing"

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
