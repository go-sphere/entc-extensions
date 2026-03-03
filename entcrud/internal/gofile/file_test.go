package gofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sphere/entc-extensions/entcrud/internal/inspect"
)

func TestFilenames_Next(t *testing.T) {
	f := NewFilenames("/tmp/test")

	// First call should return without suffix
	first := f.Next("user")
	if first != "/tmp/test/user.go" {
		t.Errorf("First call: expected /tmp/test/user.go, got %s", first)
	}

	// Second call should return with _1 suffix
	second := f.Next("user")
	if second != "/tmp/test/user_1.go" {
		t.Errorf("Second call: expected /tmp/test/user_1.go, got %s", second)
	}

	// Third call should return with _2 suffix
	third := f.Next("user")
	if third != "/tmp/test/user_2.go" {
		t.Errorf("Third call: expected /tmp/test/user_2.go, got %s", third)
	}

	// Different name should return fresh
	different := f.Next("order")
	if different != "/tmp/test/order.go" {
		t.Errorf("Different name: expected /tmp/test/order.go, got %s", different)
	}
}

func TestCreateGoFile(t *testing.T) {
	result := CreateGoFile("test", []inspect.Import{
		{Path: "fmt"},
		{Path: "strings", Alias: "strings"},
	}, "func main() {}")

	// Check that key parts are present
	if !strings.Contains(result, "package test") {
		t.Error("Missing package declaration")
	}
	if !strings.Contains(result, "import (") {
		t.Error("Missing import statement")
	}
	if !strings.Contains(result, `"fmt"`) {
		t.Error("Missing fmt import")
	}
	if !strings.Contains(result, `"strings"`) {
		t.Error("Missing strings import")
	}
	if !strings.Contains(result, "func main() {}") {
		t.Error("Missing body content")
	}
}

func TestCreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test_create_dir")

	err := CreateDir(testDir, false)
	if err != nil {
		t.Errorf("CreateDir failed: %v", err)
	}

	// Check directory exists
	info, err := os.Stat(testDir)
	if err != nil {
		t.Errorf("Failed to stat directory: %v", err)
		return
	}
	if !info.IsDir() {
		t.Error("Expected directory")
	}

	// Test with removeBeforeGenerate = true
	err = CreateDir(testDir, true)
	if err != nil {
		t.Errorf("CreateDir with remove failed: %v", err)
	}

	// Check directory still exists after remove
	info, err = os.Stat(testDir)
	if err != nil {
		t.Errorf("Failed to stat directory after remove: %v", err)
		return
	}
	if !info.IsDir() {
		t.Error("Expected directory after remove")
	}

	// Test empty dir returns error
	err = CreateDir("", false)
	if err == nil {
		t.Error("Expected error for empty directory")
	}
}
