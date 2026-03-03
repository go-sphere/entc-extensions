package main

import (
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entbind"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entmap"
)

func main() {
	_ = entmap.ToEntPost
	_ = entmap.ToProtoPost
	_ = entmap.ToEntUser
	_ = entmap.ToProtoUser
	_ = entbind.CreatePost
	_ = entbind.CreateUser
}
