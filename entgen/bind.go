package entgen

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/go-sphere/entc-extensions/entgen/conf"
	"github.com/go-sphere/entc-extensions/entgen/internal/bind"
	"github.com/go-sphere/entc-extensions/entgen/internal/gofile"
	"github.com/go-sphere/entc-extensions/entgen/internal/inspect"
	"github.com/go-sphere/entc-extensions/entgen/internal/strcase"
)

// BindFiles generates Go files for binding entities.
func BindFiles(c *conf.FilesConf) error {
	if err := gofile.CreateDir(c.Dir, c.RemoveBeforeGenerate); err != nil {
		return err
	}

	pkgName := c.Package
	if pkgName == "" {
		pkgName = "bind"
	}

	filenames := gofile.NewFilenames(c.Dir)
	{
		file := bind.CreateOptionsFile(pkgName)
		if err := gofile.WriteFile(filenames.Next("options"), []byte(file)); err != nil {
			return err
		}
	}

	for _, item := range c.Entities {
		if item.Source == nil || item.Target == nil {
			return fmt.Errorf("bind entity must provide both Source and Target types")
		}
		if len(item.Actions) == 0 {
			continue
		}
		filename := filenames.Next(strcase.ToSnake(inspect.TypeName(item.Source)))
		if err := genBindFile(filename, pkgName, c.ExtraImports, item); err != nil {
			return err
		}
	}
	return nil
}

func genBindFile(filename, pkgName string, extraImports []inspect.Import, item *conf.EntityConf) error {
	var body strings.Builder

	pkgImports := collectImports(item, extraImports)

	for _, act := range item.Actions {
		pkgImports = append(pkgImports, inspect.ExtractImport(act))
		funcContent, err := bind.GenBindFunc(act, item, item.CustomFieldConverters)
		if err != nil {
			return err
		}
		body.WriteString(funcContent)
		body.WriteString("\n")
	}

	file := gofile.CreateGoFile(pkgName, pkgImports, body.String())
	return gofile.WriteFile(filename, []byte(file))
}

func collectImports(item *conf.EntityConf, extraImports []inspect.Import) []inspect.Import {
	pkgImports := make([]inspect.Import, 0, len(extraImports)+4)

	// Add target (protobuf) package import
	pkgImports = append(pkgImports, inspect.ExtractImport(item.Target))

	// Add sub-package import for field constants (e.g., example, edgeitem)
	if subPkg := inspect.ExtractSubPackageName(item.Source); subPkg != "" {
		pkgImports = append(pkgImports, inspect.Import{
			Path:  inspect.ExtractSubPackagePath(item.Source),
			Alias: subPkg,
		})
	}

	// Extract imports from custom converters
	if item.CustomFieldConverters != nil {
		for _, converter := range item.CustomFieldConverters {
			funcInfo := inspect.GetFuncInfo(converter)
			if funcInfo.ImportPath != "" {
				pkgImports = append(pkgImports, inspect.Import{
					Path:  funcInfo.ImportPath,
					Alias: funcInfo.Package,
				})
			}
		}
	}

	// Add extra imports
	pkgImports = append(pkgImports, extraImports...)

	return pkgImports
}
