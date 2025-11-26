package inspect

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
)

// IndirectValue recursively dereferences pointers until a non-pointer value is reached.
func IndirectValue(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Ptr {
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
	if t.Kind() == reflect.Ptr {
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
	if field.Type.Kind() == reflect.Ptr {
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
	if field.Type.Kind() == reflect.Ptr {
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

// ExtractPackageImport extracts both the package path and package name from a struct value.
func ExtractPackageImport(val any) [2]string {
	return [2]string{
		ExtractPackagePath(val),
		ExtractPackageName(val),
	}
}

// DeduplicateImports removes duplicate import entries and sorts them for consistent output.
func DeduplicateImports(extraImports [][2]string) [][2]string {
	seen := make(map[[2]string]bool)
	result := make([][2]string, 0, len(extraImports))
	for _, item := range extraImports {
		if item[0] == "" {
			continue
		}
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i][0] == result[j][0] {
			return result[i][1] < result[j][1]
		}
		return result[i][0] < result[j][0]
	})
	for i, item := range result {
		if item[1] == "" {
			continue
		}
		if pkgName := path.Base(item[0]); pkgName == item[1] {
			result[i][1] = ""
		}
	}
	return result
}
