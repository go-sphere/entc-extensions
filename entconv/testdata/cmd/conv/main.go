package main

import (
	entpb "github.com/go-sphere/protocgenentconv/testdata/api/entpb"
	"github.com/go-sphere/protocgenentconv/testdata/internal/converter"
	"github.com/go-sphere/protocgenentconv/testdata/internal/pkg/database/ent"
)

func main() {
	// Test post.go
	_, _ = converter.ToEntPost(&entpb.Post{})
	_, _ = converter.ToProtoPost(&ent.Post{})

	// Test user.go
	_, _ = converter.ToEntUser(&entpb.User{})
	_, _ = converter.ToProtoUser(&ent.User{})
}
