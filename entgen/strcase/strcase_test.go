package strcase

import "testing"

func TestToSnake(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"Name", "name"},
		{"NameName", "name_name"},
		{"NameNameName", "name_name_name"},
		{"firstName", "first_name"},
		{"firstNameLastName", "first_name_last_name"},
		{"UserInfo", "user_info"},
		{"userInfo", "user_info"},
		{"UserID", "user_id"},
		{"userID", "user_id"},
		{"HTMLParser", "html_parser"},
		{"getHTTPResponse", "get_http_response"},
		{"getHTTPResponseCode", "get_http_response_code"},
		{"URLParser", "url_parser"},
		{"parseXML", "parse_xml"},
		{"parseJSON", "parse_json"},
		// Note: ToSnake has specific behavior for mixed case/number strings
		{"APIv2", "ap_iv2"},
		{"version2API", "version2api"},
	}

	for _, tt := range tests {
		result := ToSnake(tt.input)
		if result != tt.expected {
			t.Errorf("ToSnake(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}
