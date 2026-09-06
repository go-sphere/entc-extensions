package tests

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGeneratedCodeCompiles guards against generators producing code that
// passes their own unit tests but does not compile against the real generated
// ent and protobuf packages. It builds every generated render package in the
// testdata module; the entconv/entcrud outputs reference the real ent entities
// and the protoc-generated entpb package, so any field-name, type or import
// drift surfaces here.
func TestGeneratedCodeCompiles(t *testing.T) {
	root := projectRoot(t)
	packages := []string{
		"./internal/pkg/render/entmap/...",
		"./internal/pkg/render/entbind/...",
	}
	for _, pkg := range packages {
		pkg := pkg
		t.Run(filepath.Base(pkg), func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command("go", "build", pkg)
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go build %s failed (generated code does not compile):\n%s", pkg, out)
			}
		})
	}
}
