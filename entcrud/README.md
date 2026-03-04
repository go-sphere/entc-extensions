# entcrud

`entcrud` generates typed bind helpers between protobuf request structs and Ent mutation builders (for example `ent.UserCreate` / `ent.UserUpdateOne`).

## Install

```bash
go get github.com/go-sphere/entc-extensions/entcrud
```

## Quick Start

```go
package main

import (
	"log"

	entgen "github.com/go-sphere/entc-extensions/entcrud"
	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/example/project/api/entpb"
	"github.com/example/project/internal/pkg/database/ent"
	"github.com/example/project/internal/pkg/database/ent/post"
	"github.com/example/project/internal/pkg/render/entmap"
)

func main() {
	cfg := conf.NewFilesConf(
		"./internal/pkg/render/entbind",
		"entbind",
		conf.NewEntity(
			ent.Post{},
			entpb.Post{},
			[]any{ent.PostCreate{}, ent.PostUpdateOne{}},
			conf.WithCustomFieldConverter(post.FieldStatus, entmap.ToEntPost_Status),
		),
	)
	if err := entgen.BindFiles(cfg); err != nil {
		log.Fatal(err)
	}
}
```

## Strict Type Check (Default On)

`entcrud` now fails fast on known incompatible field pairs (for example `time.Time <- int64`) unless you provide a custom converter.

- Default: strict checking is enabled.
- Disable (optional): `conf.NewFilesConf(...).Apply(conf.WithStrictTypeCheck(false))`

When strict checking fails, `BindFiles` returns `*conf.TypeMismatchListError` with per-field details:

- `Entity`
- `Field`
- `SourceType`
- `TargetType`
- `Suggestion`

## Custom Field Converter

Use `conf.WithCustomFieldConverter(fieldConst, converterFunc)` to handle non-trivial mappings.

Requirements:

- converter input type must match target protobuf field type
- converter return type must match Ent field type

Example:

```go
conf.WithCustomFieldConverter(user.FieldBirthday, conv.ToEntUserBirthday)
```

## Output

`BindFiles` generates:

- `options.go` (shared bind options)
- one file per entity (for example `user.go`, `post.go`)

Generated files are `goimports` + `gofmt` formatted.

## Failure Modes

`BindFiles` returns errors for:

- missing source/target type in `EntityConf`
- template/render/write failures
- strict type mismatch without converter (`*conf.TypeMismatchListError`)

No invalid placeholder code is generated for mismatches anymore.

