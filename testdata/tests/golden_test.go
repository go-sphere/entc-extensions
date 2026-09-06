package tests

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "update complete generated-output golden files")

func TestGeneratedGoldenFiles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		generated string
		golden    string
	}{
		{name: "entproto", generated: "proto/entpb/entpb.proto", golden: "golden/entpb.proto"},
		{name: "entconv", generated: "internal/pkg/render/entmap/user.go", golden: "golden/user.entmap.golden"},
		{name: "entcrud", generated: "internal/pkg/render/entbind/user.go", golden: "golden/user.entbind.golden"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := filepath.Clean("..")
			generatedPath := filepath.Join(root, testCase.generated)
			goldenPath := filepath.Join(root, testCase.golden)

			got, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("read generated output %s: %v", generatedPath, err)
			}
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("create golden directory: %v", err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("update golden output %s: %v", goldenPath, err)
				}
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden output %s: %v", goldenPath, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("generated output differs from %s; review the change, then run make update-golden", goldenPath)
			}
		})
	}
}
