package tests

import (
	"errors"
	"path/filepath"
	"testing"

	entgen "github.com/go-sphere/entc-extensions/entcrud"
	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent"
)

func TestEntcrudStrictTypeCheck(t *testing.T) {
	cfg := conf.NewFilesConf(
		filepath.Join(t.TempDir(), "out"),
		"bind",
		conf.NewEntity(
			ent.User{},
			entpb.User{},
			[]any{ent.UserCreate{}},
		),
	)

	err := entgen.BindFiles(cfg)
	if err == nil {
		t.Fatalf("[entcrud] expected strict type check to fail without custom converter for birthday")
	}
	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		t.Fatalf("[entcrud] expected *conf.TypeMismatchListError, got %T (%v)", err, err)
	}
	if len(mismatch.Items) == 0 {
		t.Fatalf("[entcrud] expected at least one type mismatch item")
	}
}
