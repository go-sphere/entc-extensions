package tests

import (
	"path/filepath"
	"runtime"
	"testing"
)

func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve testdata root path")
	}
	// .../testdata/tests/helpers_test.go -> .../testdata
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func fromRoot(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(projectRoot(t), rel)
}
