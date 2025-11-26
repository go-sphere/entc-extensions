package gen

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sphere/entc-extensions/autoproto/gen/conf"
	"github.com/go-sphere/entc-extensions/autoproto/gen/mapper"
	"github.com/go-sphere/entc-extensions/autoproto/utils/gofile"
	"github.com/go-sphere/entc-extensions/autoproto/utils/inspect"
	"github.com/go-sphere/entc-extensions/autoproto/utils/strcase"
)

// MapperFiles generates mapper files for the provided configuration and writes them to the specified directory.
// It creates the directory if it does not exist and optionally removes its contents before generating new files.
// Returns an error if configuration is incomplete or file operations fail.
func MapperFiles(conf *conf.FilesConf) error {
	if conf.Dir == "" {
		return fmt.Errorf("directory is required")
	}
	if conf.RemoveBeforeGenerate {
		if err := os.RemoveAll(conf.Dir); err != nil {
			return fmt.Errorf("cleanup mapper dir: %w", err)
		}
	}
	if err := os.MkdirAll(conf.Dir, 0o755); err != nil {
		return fmt.Errorf("create mapper dir: %w", err)
	}
	pkgName := conf.Package
	if pkgName == "" {
		pkgName = "mapper"
	}

	for _, item := range conf.Entities {
		if item.Source == nil || item.Target == nil {
			return fmt.Errorf("mapper item must provide both Source and Target types")
		}
		filename := filepath.Join(conf.Dir, fmt.Sprintf("%s.go", strcase.ToSnake(inspect.TypeName(item.Source))))
		err := genMapperFile(filename, pkgName, conf.ExtraImports, item)
		if err != nil {
			return err
		}
	}
	return nil
}

func genMapperFile(fileName string, pkgName string, pkgImports [][2]string, item *conf.EntityConf) error {
	var body strings.Builder

	pkgImports = append(pkgImports,
		[2]string{"fmt", ""},
		inspect.ExtractPackageImport(item.Source),
		inspect.ExtractPackageImport(item.Target),
	)

	content, err := mapper.GenMapperFunc(item)
	if err != nil {
		return err
	}
	body.WriteString(content)

	file := gofile.CreateGoFile(pkgName, pkgImports, body.String())
	return gofile.WriteFile(fileName, []byte(file))
}
