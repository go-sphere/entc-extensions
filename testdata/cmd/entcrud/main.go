package main

import (
	"log"

	entgen "github.com/go-sphere/entc-extensions/entcrud"
	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/testdata/api/entpb"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/database/ent/post"
	"github.com/go-sphere/entc-extensions/testdata/internal/pkg/render/entmap"
)

func main() {
	config := conf.NewFilesConf(
		"./internal/pkg/render/entbind",
		"entbind",
		conf.NewEntity(
			ent.User{},
			entpb.User{},
			[]any{ent.UserCreate{}, ent.UserUpdateOne{}},
		),
		conf.NewEntity(
			ent.Post{},
			entpb.Post{},
			[]any{ent.PostCreate{}, ent.PostUpdateOne{}},
			conf.WithCustomFieldConverter(post.FieldStatus, entmap.ToEntPost_Status),
		),
	)
	if err := entgen.BindFiles(config); err != nil {
		log.Fatal(err)
	}
}
