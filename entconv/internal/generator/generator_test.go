package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveStaleConverters(t *testing.T) {
	dir := t.TempDir()

	generated := filepath.Join(dir, "stale.go")
	if err := os.WriteFile(generated, []byte(generatedHeader+"\npackage entmap\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handwritten := filepath.Join(dir, "custom.go")
	if err := os.WriteFile(handwritten, []byte("package entmap\n\nfunc Custom() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nonGo := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(nonGo, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeStaleConverters(dir); err != nil {
		t.Fatalf("removeStaleConverters: %v", err)
	}

	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("generated file should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(handwritten); err != nil {
		t.Fatalf("handwritten file must be kept: %v", err)
	}
	if _, err := os.Stat(nonGo); err != nil {
		t.Fatalf("non-Go file must be kept: %v", err)
	}
}
