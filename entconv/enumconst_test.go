package entconv

import "testing"

func TestPbEnumConstName(t *testing.T) {
	cases := []struct {
		msg, enum, value string
		omit             bool
		want             string
	}{
		{"Post", "Status", "PENDING", false, "Post_STATUS_PENDING"},
		{"Post", "Status", "in_progress", false, "Post_STATUS_IN_PROGRESS"},
		{"User", "OptionalLevel", "HIGH", false, "User_OPTIONAL_LEVEL_HIGH"},
		{"User", "OptionalLevel", "high", true, "User_HIGH"},
	}
	for _, c := range cases {
		if got := pbEnumConstName(c.msg, c.enum, c.value, c.omit); got != c.want {
			t.Errorf("pbEnumConstName(%q,%q,%q,%v) = %q, want %q", c.msg, c.enum, c.value, c.omit, got, c.want)
		}
	}
}

func TestEnumAnnotationOmitPrefix(t *testing.T) {
	// nil / missing annotation => false
	if enumAnnotationOmitPrefix(nil) {
		t.Fatal("nil field should not omit prefix")
	}
}
