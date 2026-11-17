package entgen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/entcrud/internal/inspect"
)

type Option struct {
	Name string
}

type optionTarget struct {
	Name string
}

type optionCreate struct{}

func (optionCreate) SetName(string) {}

func TestBindFiles_ReservesOptionAndDisambiguatesCollidingImports(t *testing.T) {
	outDir := t.TempDir()
	cfg := conf.NewFilesConf(outDir, "entbind",
		conf.NewEntity(Option{}, optionTarget{}, []any{optionCreate{}}),
	)
	cfg.ExtraImports = []inspect.Import{
		{Path: "example.com/colliding/option", Alias: "option"},
		{Path: "example.com/api/entcrud/v2", Alias: "entcrud"},
	}

	if err := BindFiles(cfg); err != nil {
		t.Fatalf("BindFiles failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "option.go"))
	if err != nil {
		t.Fatalf("read generated option.go: %v", err)
	}
	src := string(content)
	if !strings.Contains(src, "option2.FieldName") {
		t.Fatalf("expected subpackage option to be aliased as option2 to avoid shadowing local option:\n%s", src)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "option.go", src, 0); err != nil {
		t.Fatalf("generated option.go failed to parse: %v\n%s", err, src)
	}
}
