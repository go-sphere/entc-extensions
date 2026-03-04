package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	entgen "github.com/go-sphere/entc-extensions/entcrud"
	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent/post"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entbind"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entmap"
)

func main() {
	assertGeneratedSnapshots()
	assertRoundTripConversions()
	assertNegativeStrictTypeCheck()

	// keep symbol references as a compile guard
	_ = entbind.CreatePost
	_ = entbind.CreateUser
	_ = entmap.ToEntPost
	_ = entmap.ToEntUser
	_ = entmap.ToProtoPost
	_ = entmap.ToProtoUser
}

func assertGeneratedSnapshots() {
	userBindFile := mustRead("./internal/pkg/render/entbind/user.go")
	if strings.Contains(userBindFile, "TODO: add custom converter") {
		panic("generated entbind/user.go still contains TODO placeholder conversion")
	}
	if strings.Contains(userBindFile, "Skipped: incompatible types") {
		panic("generated entbind/user.go still contains skipped incompatible type marker")
	}
	if !strings.Contains(userBindFile, "func CreateUser(") {
		panic("generated entbind/user.go missing CreateUser")
	}
}

func assertRoundTripConversions() {
	now := time.Unix(1_723_456_789, 0).UTC()
	entUser := &ent.User{
		ID:       42,
		Name:     "alice",
		Age:      30,
		Active:   true,
		Score:    98.5,
		Birthday: now,
		Avatar:   []byte{1, 2, 3},
		Status:   "ok",
		Email:    "alice@example.com",
		Balance:  128,
		Role:     7,
	}
	pbUser, err := entmap.ToProtoUser(entUser)
	mustNoErr(err)
	backUser, err := entmap.ToEntUser(pbUser)
	mustNoErr(err)
	if backUser.ID != entUser.ID || backUser.Name != entUser.Name || backUser.Age != entUser.Age {
		panic(fmt.Sprintf("user round-trip mismatch: %+v -> %+v", entUser, backUser))
	}
	if backUser.Birthday.Unix() != entUser.Birthday.Unix() {
		panic(fmt.Sprintf("user birthday round-trip mismatch: %v != %v", backUser.Birthday, entUser.Birthday))
	}

	entPost := &ent.Post{
		ID:        10,
		Title:     "hello",
		Content:   "world",
		ViewCount: 99,
		Published: true,
		Status:    post.StatusInProgress,
		Likes:     7,
	}
	pbPost, err := entmap.ToProtoPost(entPost)
	mustNoErr(err)
	backPost, err := entmap.ToEntPost(pbPost)
	mustNoErr(err)
	if backPost.Status != entPost.Status || backPost.Title != entPost.Title {
		panic(fmt.Sprintf("post round-trip mismatch: %+v -> %+v", entPost, backPost))
	}
	if pbPost.Status != entpb.Post_STATUS_IN_PROGRESS {
		panic(fmt.Sprintf("expected enum conversion to STATUS_IN_PROGRESS, got %v", pbPost.Status))
	}
}

func assertNegativeStrictTypeCheck() {
	tempDir, err := os.MkdirTemp("", "entcrud-negative-*")
	mustNoErr(err)
	defer func() {
		mustNoErr(os.RemoveAll(tempDir))
	}()

	cfg := conf.NewFilesConf(
		filepath.Join(tempDir, "out"),
		"bind",
		conf.NewEntity(
			ent.User{},
			entpb.User{},
			[]any{ent.UserCreate{}},
		),
	)
	err = entgen.BindFiles(cfg)
	if err == nil {
		panic("expected strict type check to fail without custom converter for birthday")
	}
	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		panic(fmt.Sprintf("expected *conf.TypeMismatchListError, got %T (%v)", err, err))
	}
	if len(mismatch.Items) == 0 {
		panic("expected at least one type mismatch item")
	}
}

func mustRead(path string) string {
	b, err := os.ReadFile(path)
	mustNoErr(err)
	return string(b)
}

func mustNoErr(err error) {
	if err != nil {
		panic(err)
	}
}
