package entgen

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/entcrud/internal/bind"
	"github.com/go-sphere/entc-extensions/entcrud/internal/gofile"
	"github.com/go-sphere/entc-extensions/entcrud/internal/inspect"
	"github.com/go-sphere/entc-extensions/entcrud/internal/strcase"
)

// BindFiles writes the bind helper package described by c into c.Dir,
// creating the directory if needed. It first writes options.go, then one file
// per entity with at least one action, named after the snake_case Source type
// (user.go for ent.User; a repeated name gets a numeric suffix such as
// user_1.go). Existing files with those names are overwritten; with
// c.RemoveBeforeGenerate, previously generated files are deleted first.
//
// For each action type A on entity E, the generated function is named by
// moving the E prefix of A to the end (UserCreate -> CreateUser) and has the
// form
//
//	func CreateUser(source *ent.UserCreate, target *pb.User, options ...Option) *ent.UserCreate
//
// BindFiles fails if c.Dir is empty or a symlink, if an entity has a nil
// Source or Target, or, with c.StrictTypeCheck, with a [*conf.TypeMismatchListError]
// for the first action that has incompatible fields. It stops at the first
// error; files written before it remain. c must not be nil and an empty
// c.Package means "bind".
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
		if err := genBindFile(filename, pkgName, c.ExtraImports, item, c.StrictTypeCheck); err != nil {
			return err
		}
	}
	return nil
}

func genBindFile(filename, pkgName string, extraImports []inspect.Import, item *conf.EntityConf, strictTypeCheck bool) error {
	var body strings.Builder

	pkgImports := collectImports(item, extraImports)

	for _, act := range item.Actions {
		pkgImports = append(pkgImports, inspect.ExtractImport(act))
		funcContent, err := bind.GenBindFunc(act, item, item.CustomFieldConverters, strictTypeCheck)
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
