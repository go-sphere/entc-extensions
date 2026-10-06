package conf_test

import (
	"fmt"

	"github.com/go-sphere/entc-extensions/entcrud/conf"
)

type (
	User        struct{ Name, PasswordHash string }
	UserCreate  struct{}
	UserMessage struct{ Name, PasswordHash string }
)

func ExampleNewFilesConf() {
	cfg := conf.NewFilesConf("./internal/entbind", "entbind",
		conf.NewEntity(User{}, UserMessage{}, []any{UserCreate{}},
			conf.WithIgnoreFields("password_hash"),
		),
	).Apply(conf.WithStrictTypeCheck(false))

	fmt.Println(cfg.Dir, cfg.Package, cfg.StrictTypeCheck, cfg.RemoveBeforeGenerate)
	fmt.Println(cfg.Entities[0].IgnoreFields)
	// Output:
	// ./internal/entbind entbind false false
	// [password_hash]
}
