package inspect

import (
	"fmt"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

// IndirectValue recursively dereferences pointers until a non-pointer value is reached.
func IndirectValue(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	return value
}

// TypeName returns the name of the most deeply dereferenced type of the given value.
func TypeName(value any) string {
	return IndirectValue(reflect.ValueOf(value)).Type().Name()
}

// ExtractTypeName returns the simple type name without package prefix.
func ExtractTypeName(val any) string {
	value := IndirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	return typeOf.Name()
}

// ExtractPackagePath returns the full import path of the package containing the type.
func ExtractPackagePath(val any) string {
	value := IndirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	return typeOf.PkgPath()
}

// ExtractPackageName extracts the package name from a struct value's type information.
func ExtractPackageName(val any) string {
	value := IndirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	fullName := typeOf.String()
	if !strings.Contains(fullName, ".") {
		return ""
	}
	parts := strings.Split(fullName, ".")
	return parts[0]
}

// ExtractSubPackageName extracts the sub-package name from a struct value's type information.
// For example, for ent.Example, it returns "example".
func ExtractSubPackageName(val any) string {
	value := IndirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	typeName := typeOf.Name()
	// typeName examples:
	// Example -> "example"
	// EdgeItem -> "edgeitem"
	return strings.ToLower(typeName)
}

// ExtractSubPackagePath extracts the full import path to the sub-package from a struct value's type information.
// For example, for ent.Example, it returns "github.com/xxx/ent/example".
func ExtractSubPackagePath(val any) string {
	value := IndirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	fullName := typeOf.String()
	// fullName examples:
	// ent.Example -> "example"
	// ent.EdgeItem -> "edgeitem"
	// *ent.Example -> "example"
	parts := strings.Split(fullName, ".")
	if len(parts) >= 2 {
		subPkg := strings.ToLower(parts[len(parts)-1])
		pkgPath := typeOf.PkgPath()
		// pkgPath is like "github.com/xxx/ent", we need to append the subpackage
		return pkgPath + "/" + subPkg
	}
	return ""
}

// ExtractPublicFields extracts all public (exported) fields from a struct using reflection.
func ExtractPublicFields(obj any, keyMapper func(string) string) ([]string, map[string]reflect.StructField) {
	if obj == nil {
		return nil, nil
	}
	val := IndirectValue(reflect.ValueOf(obj))
	if val.Kind() != reflect.Struct {
		return nil, nil
	}
	typ := val.Type()
	keys := make([]string, 0, typ.NumField())
	fields := make(map[string]reflect.StructField)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() || field.Anonymous {
			continue
		}
		key := field.Name
		if keyMapper != nil {
			key = keyMapper(key)
		}
		keys = append(keys, key)
		fields[key] = field
	}
	return keys, fields
}

// ExtractPublicMethods extracts all public methods from a type using reflection.
func ExtractPublicMethods(obj any, keyMapper func(string) string) ([]string, map[string]reflect.Method) {
	if obj == nil {
		return nil, nil
	}
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Interface {
		return nil, nil
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	types := []reflect.Type{t, reflect.PointerTo(t)}
	keys := make([]string, 0)
	methods := make(map[string]reflect.Method)
	seen := make(map[string]struct{})

	for _, typ := range types {
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			if !m.IsExported() {
				continue
			}
			name := m.Name
			if keyMapper != nil {
				name = keyMapper(name)
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			keys = append(keys, name)
			methods[name] = m
		}
	}
	return keys, methods
}

// GenerateZeroCheckExpr generates Go code that checks if a struct field contains its zero value.
func GenerateZeroCheckExpr(sourceName string, field reflect.StructField) string {
	if field.Type.Kind() == reflect.Pointer {
		return fmt.Sprintf("%s.%s == nil", sourceName, field.Name)
	}
	switch field.Type.Kind() {
	case reflect.String:
		return fmt.Sprintf("%s.%s == \"\"", sourceName, field.Name)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%s.%s == 0", sourceName, field.Name)
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%s.%s == 0.0", sourceName, field.Name)
	case reflect.Bool:
		return fmt.Sprintf("!%s.%s", sourceName, field.Name)
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		return fmt.Sprintf("%s.%s == nil", sourceName, field.Name)
	default:
		return fmt.Sprintf("reflect.ValueOf(%s.%s).IsZero()", sourceName, field.Name)
	}
}

// GenerateNonZeroCheckExpr generates Go code that checks if a struct field contains a non-zero value.
func GenerateNonZeroCheckExpr(sourceName string, field reflect.StructField) string {
	if field.Type.Kind() == reflect.Pointer {
		return fmt.Sprintf("%s.%s != nil", sourceName, field.Name)
	}
	switch field.Type.Kind() {
	case reflect.String:
		return fmt.Sprintf("%s.%s != \"\"", sourceName, field.Name)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%s.%s != 0", sourceName, field.Name)
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%s.%s != 0.0", sourceName, field.Name)
	case reflect.Bool:
		return fmt.Sprintf("%s.%s", sourceName, field.Name)
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		return fmt.Sprintf("%s.%s != nil", sourceName, field.Name)
	default:
		return fmt.Sprintf("!reflect.ValueOf(%s.%s).IsZero()", sourceName, field.Name)
	}
}

// Import represents a Go import with path and optional alias.
type Import struct {
	Path  string
	Alias string
}

// ExtractImport extracts both the package path and package name from a struct value.
func ExtractImport(val any) Import {
	return Import{
		Path:  ExtractPackagePath(val),
		Alias: ExtractPackageName(val),
	}
}

// ExtractPackageImport extracts both the package path and package name from a struct value.
// Deprecated: Use ExtractImport instead.
func ExtractPackageImport(val any) Import {
	return ExtractImport(val)
}

// DeduplicateImports removes duplicate import entries and sorts them for consistent output.
func DeduplicateImports(extraImports []Import) []Import {
	seen := make(map[Import]bool)
	result := make([]Import, 0, len(extraImports))
	for _, item := range extraImports {
		if item.Path == "" {
			continue
		}
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Alias < result[j].Alias
		}
		return result[i].Path < result[j].Path
	})
	for i, item := range result {
		if item.Alias == "" {
			continue
		}
		if pkgName := path.Base(item.Path); pkgName == item.Alias {
			result[i].Alias = ""
		}
	}
	return result
}

// FuncInfo holds metadata about a function that can be used for code generation.
type FuncInfo struct {
	ImportPath string // e.g., github.com/xxx/project/entconv
	Package    string // e.g., entconv
	Name       string // e.g., GenerateConverterFile / GenerateConverterFile.func1
	FullName   string // runtime original name
}

// GetFuncInfo extracts function metadata from a function value.
// It returns a FuncInfo struct containing the import path, package name,
// function name, and full runtime name.
func GetFuncInfo(fn any) FuncInfo {
	v := reflect.ValueOf(fn)
	if !v.IsValid() || v.Kind() != reflect.Func {
		return FuncInfo{}
	}

	pc := v.Pointer()
	rf := runtime.FuncForPC(pc)
	if rf == nil {
		return FuncInfo{}
	}

	full := rf.Name()
	// full examples:
	// github.com/xxx/project/entconv.GenerateConverterFile
	// github.com/xxx/project/entconv.(*T).Method
	// github.com/xxx/project/entconv.GenerateConverterFile.func1

	// Find the boundary between "package path + package name" and "symbol name":
	// That is: after the last "/", find the first "."
	lastSlash := strings.LastIndex(full, "/")
	searchFrom := 0
	if lastSlash != -1 {
		searchFrom = lastSlash + 1
	}
	firstDotAfterSlash := strings.Index(full[searchFrom:], ".")
	if firstDotAfterSlash == -1 {
		// Rare fallback: no dot, cannot split
		return FuncInfo{FullName: full, Name: full}
	}
	firstDotAfterSlash += searchFrom

	importPath := full[:firstDotAfterSlash] // up to end of package name (excluding dot)
	symbol := full[firstDotAfterSlash+1:]   // everything after the dot: function name/method/closure

	// Package name: after last "/" in importPath
	pkg := importPath
	if i := strings.LastIndex(importPath, "/"); i != -1 {
		pkg = importPath[i+1:]
	}

	return FuncInfo{
		ImportPath: importPath,
		Package:    pkg,
		Name:       symbol,
		FullName:   full,
	}
}
