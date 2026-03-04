package tests

import (
	"testing"
	"time"

	"github.com/go-sphere/entc-extensions/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent/post"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entbind"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entmap"
)

func TestGeneratedSymbolsExist(t *testing.T) {
	_ = entbind.CreateGroup
	_ = entbind.UpdateOneGroup
	_ = entbind.CreatePost
	_ = entbind.CreateUser
	_ = entmap.ToEntGroup
	_ = entmap.ToProtoGroup
	_ = entmap.ToEntPost
	_ = entmap.ToEntUser
	_ = entmap.ToProtoPost
	_ = entmap.ToProtoUser
}

func TestEntconvRoundTrip(t *testing.T) {
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
		Rank:     3,
		Quota:    4096,
		Tags:     []string{"staff", "beta"},
		Points:   []int64{9, 8, 7},
	}

	pbUser, err := entmap.ToProtoUser(entUser)
	if err != nil {
		t.Fatalf("[entconv] ToProtoUser failed: %v", err)
	}
	backUser, err := entmap.ToEntUser(pbUser)
	if err != nil {
		t.Fatalf("[entconv] ToEntUser failed: %v", err)
	}
	if pbUser == nil || backUser == nil {
		t.Fatalf("[entconv] unexpected nil user conversion result: pb=%v back=%v", pbUser, backUser)
	}

	if backUser.ID != entUser.ID || backUser.Name != entUser.Name || backUser.Age != entUser.Age {
		t.Fatalf("[entconv] user round-trip mismatch: %+v -> %+v", entUser, backUser)
	}
	if backUser.Birthday.Unix() != entUser.Birthday.Unix() {
		t.Fatalf("[entconv] user birthday round-trip mismatch: %v != %v", backUser.Birthday, entUser.Birthday)
	}
	if backUser.Rank != entUser.Rank || backUser.Quota != entUser.Quota {
		t.Fatalf("[entconv] user rank/quota round-trip mismatch: %+v -> %+v", entUser, backUser)
	}
	if len(backUser.Tags) != 2 || len(backUser.Points) != 3 {
		t.Fatalf("[entconv] user json fields round-trip mismatch: tags=%v points=%v", backUser.Tags, backUser.Points)
	}

	entPost := &ent.Post{
		ID:        10,
		Title:     "hello",
		Content:   "world",
		ViewCount: 99,
		Published: true,
		Status:    post.StatusInProgress,
		Likes:     7,
		Shares:    11,
	}
	pbPost, err := entmap.ToProtoPost(entPost)
	if err != nil {
		t.Fatalf("[entconv] ToProtoPost failed: %v", err)
	}
	backPost, err := entmap.ToEntPost(pbPost)
	if err != nil {
		t.Fatalf("[entconv] ToEntPost failed: %v", err)
	}
	if pbPost == nil || backPost == nil {
		t.Fatalf("[entconv] unexpected nil post conversion result: pb=%v back=%v", pbPost, backPost)
	}

	if backPost.Status != entPost.Status || backPost.Title != entPost.Title {
		t.Fatalf("[entconv] post round-trip mismatch: %+v -> %+v", entPost, backPost)
	}
	if pbPost.Status != entpb.Post_STATUS_IN_PROGRESS {
		t.Fatalf("[entconv] expected enum conversion to STATUS_IN_PROGRESS, got %v", pbPost.Status)
	}
	if backPost.Shares != entPost.Shares {
		t.Fatalf("[entconv] post shares round-trip mismatch: %d != %d", backPost.Shares, entPost.Shares)
	}

	entGroup := &ent.Group{
		ID:     77,
		Name:   "core",
		Active: true,
		Labels: []string{"platform", "ops"},
	}
	pbGroup, err := entmap.ToProtoGroup(entGroup)
	if err != nil {
		t.Fatalf("[entconv] ToProtoGroup failed: %v", err)
	}
	backGroup, err := entmap.ToEntGroup(pbGroup)
	if err != nil {
		t.Fatalf("[entconv] ToEntGroup failed: %v", err)
	}
	if pbGroup == nil || backGroup == nil {
		t.Fatalf("[entconv] unexpected nil group conversion result: pb=%v back=%v", pbGroup, backGroup)
	}
	if backGroup.Name != entGroup.Name || len(backGroup.Labels) != len(entGroup.Labels) {
		t.Fatalf("[entconv] group round-trip mismatch: %+v -> %+v", entGroup, backGroup)
	}
}
