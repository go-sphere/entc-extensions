package pkgutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetModulePath(t *testing.T) {
	cases := []struct {
		name    string
		gomod   string
		want    string
		wantErr bool
	}{
		{
			name:  "standard",
			gomod: "module example.com/app\n\ngo 1.26\n",
			want:  "example.com/app",
		},
		{
			name:  "quoted with v2 and comment",
			gomod: "// leading comment\nmodule \"example.com/app/v2\" // trailing\n\ngo 1.26\n",
			want:  "example.com/app/v2",
		},
		{
			name:  "tab separator",
			gomod: "module\texample.com/tabbed\n",
			want:  "example.com/tabbed",
		},
		{
			name:    "missing module directive",
			gomod:   "go 1.26\n",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(tc.gomod), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := GetModulePath(dir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetModulePath: %v", err)
			}
			if got != tc.want {
				t.Fatalf("GetModulePath = %q, want %q", got, tc.want)
			}
		})
	}

	if _, err := GetModulePath(t.TempDir()); err == nil {
		t.Fatal("expected error for missing go.mod")
	}
}

func TestFindModuleRootAndDiskDirForImportPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/proj/v2\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "internal", "ent")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	foundRoot, err := FindModuleRoot(sub)
	if err != nil {
		t.Fatalf("FindModuleRoot: %v", err)
	}
	if foundRoot != root {
		t.Fatalf("FindModuleRoot = %q, want %q", foundRoot, root)
	}

	dir, err := DiskDirForImportPath(sub, "example.com/proj/v2/internal/ent")
	if err != nil {
		t.Fatalf("DiskDirForImportPath: %v", err)
	}
	if dir != sub {
		t.Fatalf("DiskDirForImportPath = %q, want %q", dir, sub)
	}

	rootDir, err := DiskDirForImportPath(sub, "example.com/proj/v2")
	if err != nil {
		t.Fatalf("DiskDirForImportPath(root): %v", err)
	}
	if rootDir != root {
		t.Fatalf("DiskDirForImportPath(root) = %q, want %q", rootDir, root)
	}

	if _, err := DiskDirForImportPath(sub, "example.com/proj/v20"); err == nil {
		t.Fatal("expected error for non-boundary prefix match")
	}
}

func TestResolveEntPackage(t *testing.T) {
	got, err := ResolveEntPackage("./schema", "example.com/custom/ent")
	if err != nil {
		t.Fatalf("ResolveEntPackage explicit: %v", err)
	}
	if got != "example.com/custom/ent" {
		t.Fatalf("ResolveEntPackage = %q, want example.com/custom/ent", got)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/proj\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	schemaDir := filepath.Join(root, "internal", "db", "schema")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveEntPackage(schemaDir, "")
	if err != nil {
		t.Fatalf("ResolveEntPackage derived: %v", err)
	}
	if got != "example.com/proj/internal/db/ent" {
		t.Fatalf("ResolveEntPackage = %q, want example.com/proj/internal/db/ent", got)
	}
}
