package entproto

import (
	"strings"
	"testing"
)

func TestSkippedSchemasWarning(t *testing.T) {
	got := skippedSchemasWarning([]string{"User", "Group"})
	if !strings.Contains(got, "Group, User") {
		t.Fatalf("warning does not list all schemas in sorted order: %s", got)
	}
	if !strings.Contains(got, "entproto.Message") {
		t.Fatalf("warning does not mention the missing annotation: %s", got)
	}
}

func TestSkippedSchemasWarning_SortsInput(t *testing.T) {
	got := skippedSchemasWarning([]string{"Zeta", "Alpha"})
	if !strings.Contains(got, "Alpha, Zeta") {
		t.Fatalf("warning is not sorted: %s", got)
	}
}
