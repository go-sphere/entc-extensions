package main

import (
	"github.com/go-sphere/protocgenentconv/testdata/api/entpb"
	"github.com/go-sphere/protocgenentconv/testdata/internal/pkg/database/ent"
)

func main() {
	_, _ = entpb.ToEntPost(&entpb.Post{})
	_, _ = entpb.ToProtoUser(&ent.User{})
}
