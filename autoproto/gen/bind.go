package gen

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sphere/entc-extensions/autoproto/gen/bind"
	"github.com/go-sphere/entc-extensions/autoproto/gen/conf"
	"github.com/go-sphere/entc-extensions/autoproto/utils/gofile"
	"github.com/go-sphere/entc-extensions/autoproto/utils/inspect"
	"github.com/go-sphere/entc-extensions/autoproto/utils/strcase"
)

// BindFiles generates Go files to map entities as per the provided configuration and writes them to the specified directory.
// It supports options to clean the directory before generation and allows specifying additional imports and package names.
// Returns an error if directory operations, file writing, or file generation fail.
func BindFiles(conf *conf.FilesConf) error {
	if conf.Dir == "" {
		return fmt.Errorf("directory is required")
	}
	if conf.RemoveBeforeGenerate {
		if err := os.RemoveAll(conf.Dir); err != nil {
			return fmt.Errorf("cleanup bind dir: %w", err)
		}
	}
	if err := os.MkdirAll(conf.Dir, 0o755); err != nil {
		return fmt.Errorf("create bind dir: %w", err)
	}
	pkgName := conf.Package
	if pkgName == "" {
		pkgName = "bind"
	}

	for _, item := range conf.Entities {
		if item.Source == nil || item.Target == nil {
			return fmt.Errorf("bind entity must provide both Source and Target types")
		}
		filename := filepath.Join(conf.Dir, fmt.Sprintf("%s.go", strcase.ToSnake(inspect.TypeName(item.Source))))
		err := genBindFile(filename, pkgName, conf.ExtraImports, item)
		if err != nil {
			return err
		}
	}
	return nil
}

func genBindFile(fileName string, pkgName string, pkgImports [][2]string, item *conf.EntityConf) error {
	var body strings.Builder

	pkgImports = append(pkgImports,
		inspect.ExtractPackageImport(bind.Options{}),
		inspect.ExtractPackageImport(item.Source),
		inspect.ExtractPackageImport(item.Target),
	)

	for _, act := range item.Actions {
		pkgImports = append(pkgImports,
			inspect.ExtractPackageImport(act),
		)
		funcContent, err := bind.GenBindFunc(act, item)
		if err != nil {
			return err
		}
		body.WriteString(funcContent)
		body.WriteString("\n")
	}

	file := gofile.CreateGoFile(pkgName, pkgImports, body.String())
	err := gofile.WriteFile(fileName, []byte(file))
	if err != nil {
		return err
	}
	return nil
}
