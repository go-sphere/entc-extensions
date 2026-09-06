package tests

import (
	"testing"
	"time"

	"github.com/go-sphere/entc-extensions/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent/post"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent/user"
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
		ID:               42,
		Name:             "alice",
		Age:              30,
		Active:           true,
		Score:            98.5,
		Birthday:         now,
		Avatar:           []byte{1, 2, 3},
		Status:           "ok",
		Email:            "alice@example.com",
		Balance:          128,
		Role:             7,
		Rank:             3,
		Quota:            4096,
		Tags:             []string{"staff", "beta"},
		Points:           []int64{9, 8, 7},
		OptionalBlob:     new([]byte{4, 5, 6}),
		OptionalCount:    new(12),
		OptionalBirthday: new(now),
		OptionalLevel:    new(user.OptionalLevelHigh),
		OptionalNote:     new("memo"),
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
	if pbUser.OptionalNote == nil || *pbUser.OptionalNote != *entUser.OptionalNote {
		t.Fatalf("[entconv] optional note was not preserved in protobuf: ent=%v pb=%v", entUser.OptionalNote, pbUser.OptionalNote)
	}
	if backUser.OptionalNote == nil || *backUser.OptionalNote != *entUser.OptionalNote {
		t.Fatalf("[entconv] optional note round-trip mismatch: ent=%v back=%v", entUser.OptionalNote, backUser.OptionalNote)
	}
	if pbUser.OptionalBlob == nil || string(pbUser.OptionalBlob) != string(*entUser.OptionalBlob) {
		t.Fatalf("[entconv] optional blob was not preserved in protobuf: ent=%v pb=%v", entUser.OptionalBlob, pbUser.OptionalBlob)
	}
	if backUser.OptionalBlob == nil || string(*backUser.OptionalBlob) != string(*entUser.OptionalBlob) {
		t.Fatalf("[entconv] optional blob round-trip mismatch: ent=%v back=%v", entUser.OptionalBlob, backUser.OptionalBlob)
	}
	if pbUser.OptionalCount == nil || *pbUser.OptionalCount != int64(*entUser.OptionalCount) ||
		backUser.OptionalCount == nil || *backUser.OptionalCount != *entUser.OptionalCount {
		t.Fatalf("[entconv] optional numeric round-trip mismatch: ent=%v pb=%v back=%v", entUser.OptionalCount, pbUser.OptionalCount, backUser.OptionalCount)
	}
	if pbUser.OptionalBirthday == nil || *pbUser.OptionalBirthday != entUser.OptionalBirthday.Unix() ||
		backUser.OptionalBirthday == nil || backUser.OptionalBirthday.Unix() != entUser.OptionalBirthday.Unix() {
		t.Fatalf("[entconv] optional time round-trip mismatch: ent=%v pb=%v back=%v", entUser.OptionalBirthday, pbUser.OptionalBirthday, backUser.OptionalBirthday)
	}
	if pbUser.OptionalLevel == nil || *pbUser.OptionalLevel != entpb.User_OPTIONAL_LEVEL_HIGH ||
		backUser.OptionalLevel == nil || *backUser.OptionalLevel != *entUser.OptionalLevel {
		t.Fatalf("[entconv] optional enum round-trip mismatch: ent=%v pb=%v back=%v", entUser.OptionalLevel, pbUser.OptionalLevel, backUser.OptionalLevel)
	}

	withoutNote, err := entmap.ToProtoUser(&ent.User{})
	if err != nil {
		t.Fatalf("[entconv] ToProtoUser without optional note failed: %v", err)
	}
	if withoutNote == nil {
		t.Fatal("[entconv] ToProtoUser without optional note returned nil")
	}
	if withoutNote.OptionalNote != nil {
		t.Fatalf("[entconv] nil optional note became present: %q", *withoutNote.OptionalNote)
	}
	if withoutNote.OptionalBlob != nil {
		t.Fatalf("[entconv] nil optional blob became present: %v", withoutNote.OptionalBlob)
	}
	if withoutNote.OptionalCount != nil || withoutNote.OptionalBirthday != nil || withoutNote.OptionalLevel != nil {
		t.Fatalf("[entconv] nil optional scalar became present: count=%v birthday=%v level=%v", withoutNote.OptionalCount, withoutNote.OptionalBirthday, withoutNote.OptionalLevel)
	}
	backWithoutNote, err := entmap.ToEntUser(withoutNote)
	if err != nil {
		t.Fatalf("[entconv] ToEntUser without optional note failed: %v", err)
	}
	if backWithoutNote == nil {
		t.Fatal("[entconv] ToEntUser without optional note returned nil")
	}
	if backWithoutNote.OptionalNote != nil {
		t.Fatalf("[entconv] nil optional note was not preserved: %q", *backWithoutNote.OptionalNote)
	}
	if backWithoutNote.OptionalBlob != nil {
		t.Fatalf("[entconv] nil optional blob was not preserved: %v", *backWithoutNote.OptionalBlob)
	}
	if backWithoutNote.OptionalCount != nil || backWithoutNote.OptionalBirthday != nil || backWithoutNote.OptionalLevel != nil {
		t.Fatalf("[entconv] nil optional scalar was not preserved: count=%v birthday=%v level=%v", backWithoutNote.OptionalCount, backWithoutNote.OptionalBirthday, backWithoutNote.OptionalLevel)
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
