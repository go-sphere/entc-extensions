package tests

import (
	"errors"
	"path/filepath"
	"testing"

	entgen "github.com/go-sphere/entc-extensions/entcrud"
	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent/post"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entmap"
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

func TestEntcrudStrictTypeCheck_PostIPV4(t *testing.T) {
	cfg := conf.NewFilesConf(
		filepath.Join(t.TempDir(), "out"),
		"bind",
		conf.NewEntity(
			ent.Post{},
			entpb.Post{},
			[]any{ent.PostCreate{}},
			conf.WithCustomFieldConverter(post.FieldStatus, entmap.ToEntPost_Status),
		),
	)

	err := entgen.BindFiles(cfg)
	if err == nil {
		t.Fatalf("[entcrud] expected strict type check to fail without custom converter for ip_v4")
	}
	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		t.Fatalf("[entcrud] expected *conf.TypeMismatchListError, got %T (%v)", err, err)
	}
	if len(mismatch.Items) != 1 || mismatch.Items[0].Field != "IPV4" {
		t.Fatalf("[entcrud] expected IPV4 mismatch item, got %+v", mismatch.Items)
	}
}
