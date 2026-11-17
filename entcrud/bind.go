package entgen

import (
	_ "embed"
	"fmt"
	"reflect"
	"sort"
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

	resolver := buildImportResolver(item, extraImports)

	for _, act := range item.Actions {
		funcContent, err := bind.GenBindFuncWithResolver(act, item, item.CustomFieldConverters, strictTypeCheck, resolver)
		if err != nil {
			return err
		}
		body.WriteString(funcContent)
		body.WriteString("\n")
	}

	file := gofile.CreateGoFile(pkgName, resolver.Imports(), body.String())
	return gofile.WriteFile(filename, []byte(file))
}

func buildImportResolver(item *conf.EntityConf, extraImports []inspect.Import) *inspect.ImportResolver {
	// Reserve parameter and local variable names declared by func.tmpl so no
	// imported package alias (e.g. a schema named Option or Target) shadows them.
	resolver := inspect.NewImportResolver("source", "target", "options", "option")

	for _, act := range item.Actions {
		imp := inspect.ExtractImport(act)
		resolver.Resolve(imp.Path, imp.Alias)
	}

	targetImp := inspect.ExtractImport(item.Target)
	resolver.Resolve(targetImp.Path, targetImp.Alias)

	if subPkg := inspect.ExtractSubPackageName(item.Source); subPkg != "" {
		resolver.Resolve(inspect.ExtractSubPackagePath(item.Source), subPkg)
	}

	if len(item.CustomFieldConverters) > 0 {
		keys := make([]string, 0, len(item.CustomFieldConverters))
		for k := range item.CustomFieldConverters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			funcInfo := inspect.GetFuncInfo(item.CustomFieldConverters[k])
			if funcInfo.ImportPath != "" {
				resolver.Resolve(funcInfo.ImportPath, funcInfo.Package)
			}
		}
	}

	for _, act := range item.Actions {
		for _, imp := range setterImports(act) {
			resolver.Resolve(imp.Path, imp.Alias)
		}
	}

	for _, imp := range extraImports {
		if imp.Path != "" {
			resolver.Resolve(imp.Path, imp.Alias)
		}
	}

	return resolver
}

// setterImports returns the packages of named setter argument types declared
// outside the action's own package, e.g. model.Nickname for a field declared
// with field.String(...).GoType(model.Nickname("")). The generated conversion
// references such types by name; goimports drops the ones left unused.
func setterImports(action any) []inspect.Import {
	var out []inspect.Import
	t := reflect.PointerTo(reflect.TypeOf(action))
	for i := range t.NumMethod() {
		m := t.Method(i)
		if !strings.HasPrefix(m.Name, "Set") || m.Type.NumIn() != 2 {
			continue
		}
		arg := m.Type.In(1)
		for arg.Kind() == reflect.Pointer || arg.Kind() == reflect.Slice {
			arg = arg.Elem()
		}
		pkgName, _, ok := strings.Cut(arg.String(), ".")
		if arg.PkgPath() == "" || !ok {
			continue
		}
		out = append(out, inspect.Import{Path: arg.PkgPath(), Alias: pkgName})
	}
	return out
}
