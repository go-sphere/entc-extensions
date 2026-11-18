// Package inspect extracts type names, import paths, fields, and methods from
// Go values by reflection for entcrud.
package inspect

import (
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strconv"
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
	if value == nil {
		return ""
	}
	v := IndirectValue(reflect.ValueOf(value))
	if !v.IsValid() {
		return ""
	}
	return v.Type().Name()
}

// ExtractPackagePath returns the full import path of the package containing the type.
func ExtractPackagePath(val any) string {
	if val == nil {
		return ""
	}
	value := IndirectValue(reflect.ValueOf(val))
	if !value.IsValid() {
		return ""
	}
	return value.Type().PkgPath()
}

// ExtractPackageName extracts the package name from a struct value's type information.
func ExtractPackageName(val any) string {
	if val == nil {
		return ""
	}
	value := IndirectValue(reflect.ValueOf(val))
	if !value.IsValid() {
		return ""
	}
	pkg, _, ok := strings.Cut(value.Type().String(), ".")
	if !ok {
		return ""
	}
	return pkg
}

// ExtractSubPackageName extracts the sub-package name from a struct value's type information.
// For example, for ent.Example, it returns "example".
func ExtractSubPackageName(val any) string {
	typeName := TypeName(val)
	if typeName == "" {
		return ""
	}
	return strings.ToLower(typeName)
}

// ExtractSubPackagePath extracts the full import path to the sub-package from a struct value's type information.
// For example, for ent.Example, it returns "github.com/xxx/ent/example".
func ExtractSubPackagePath(val any) string {
	pkgPath := ExtractPackagePath(val)
	subPkg := ExtractSubPackageName(val)
	if pkgPath == "" || subPkg == "" {
		return ""
	}
	return path.Join(pkgPath, subPkg)
}

// ExtractPublicFields extracts all public (exported) fields from a struct using reflection.
// Each field's primary key prefers a non-empty, non-"-" protobuf tag name,
// then the json tag name, falling back to keyMapper(field.Name) (or field.Name
// when keyMapper is nil). When the mapped field name differs from the primary
// key, it is also recorded in the returned map if not already taken.
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
		alt := field.Name
		if keyMapper != nil {
			alt = keyMapper(alt)
		}
		key := canonicalFieldKey(field)
		if key == "" {
			key = alt
		}
		keys = append(keys, key)
		fields[key] = field
		if alt != key {
			if _, exists := fields[alt]; !exists {
				fields[alt] = field
			}
		}
	}
	return keys, fields
}

func canonicalFieldKey(field reflect.StructField) string {
	if pbTag := field.Tag.Get("protobuf"); pbTag != "" && pbTag != "-" {
		for _, part := range strings.Split(pbTag, ",") {
			if name, ok := strings.CutPrefix(part, "name="); ok {
				if name != "" && name != "-" {
					return name
				}
				break
			}
		}
	}
	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		name, _, _ := strings.Cut(jsonTag, ",")
		if name != "" && name != "-" {
			return name
		}
	}
	return ""
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
	return generateCheckExpr(sourceName, field, true)
}

// GenerateNonZeroCheckExpr generates Go code that checks if a struct field contains a non-zero value.
func GenerateNonZeroCheckExpr(sourceName string, field reflect.StructField) string {
	return generateCheckExpr(sourceName, field, false)
}

func generateCheckExpr(sourceName string, field reflect.StructField, wantZero bool) string {
	expr := fmt.Sprintf("%s.%s", sourceName, field.Name)

	switch field.Type.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface, reflect.Chan, reflect.Func:
		if wantZero {
			return expr + " == nil"
		}
		return expr + " != nil"
	case reflect.String:
		if wantZero {
			return expr + ` == ""`
		}
		return expr + ` != ""`
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if wantZero {
			return expr + " == 0"
		}
		return expr + " != 0"
	case reflect.Float32, reflect.Float64:
		if wantZero {
			return expr + " == 0.0"
		}
		return expr + " != 0.0"
	case reflect.Complex64, reflect.Complex128:
		if wantZero {
			return expr + " == 0"
		}
		return expr + " != 0"
	case reflect.Bool:
		if wantZero {
			return "!" + expr
		}
		return expr
	case reflect.Array, reflect.Struct:
		if wantZero {
			return fmt.Sprintf("reflect.ValueOf(%s).IsZero()", expr)
		}
		return fmt.Sprintf("!reflect.ValueOf(%s).IsZero()", expr)
	default:
		if wantZero {
			return fmt.Sprintf("reflect.ValueOf(%s).IsZero()", expr)
		}
		return fmt.Sprintf("!reflect.ValueOf(%s).IsZero()", expr)
	}
}

// GenerateTypeConversionExpr generates Go code for type conversion expression.
func GenerateTypeConversionExpr(targetField reflect.StructField, sourceType reflect.Type) string {
	return GenerateTypeConversionExprWithAlias(targetField, sourceType, "")
}

// GenerateTypeConversionExprWithAlias generates Go code for type conversion
// expression, rewriting the source type's package qualifier to pkgAlias when
// non-empty.
func GenerateTypeConversionExprWithAlias(targetField reflect.StructField, sourceType reflect.Type, pkgAlias string) string {
	value := "target." + targetField.Name
	if targetField.Type.Kind() == reflect.Pointer && sourceType.Kind() != reflect.Pointer {
		value = "*" + value
	}
	typeExpr := sourceType.String()
	if pkgAlias != "" {
		typeExpr = rewriteTypePkgAlias(typeExpr, sourceType, pkgAlias)
	}
	return fmt.Sprintf("%s(%s)", typeExpr, value)
}

func rewriteTypePkgAlias(typeExpr string, sourceType reflect.Type, pkgAlias string) string {
	elem := sourceType
	prefix := ""
	for elem.Kind() == reflect.Pointer || elem.Kind() == reflect.Slice {
		if elem.Kind() == reflect.Pointer {
			prefix += "*"
		} else {
			prefix += "[]"
		}
		elem = elem.Elem()
	}
	origPkg, typeName, ok := strings.Cut(elem.String(), ".")
	if !ok || origPkg == "" || origPkg == pkgAlias {
		return typeExpr
	}
	if strings.HasPrefix(typeExpr, prefix+origPkg+".") {
		return prefix + pkgAlias + "." + typeName
	}
	return typeExpr
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

// ImportResolver allocates collision-free Go import aliases for a single
// generated file while keeping reserved identifier names (such as function
// parameters and local variables) free from shadowing.
type ImportResolver struct {
	used    map[string]struct{}
	byPath  map[string]string
	imports []Import
}

// NewImportResolver returns an ImportResolver with reserved identifiers marked
// unavailable as package aliases.
func NewImportResolver(reserved ...string) *ImportResolver {
	used := make(map[string]struct{}, len(reserved))
	for _, r := range reserved {
		if r != "" {
			used[r] = struct{}{}
		}
	}
	return &ImportResolver{
		used:   used,
		byPath: make(map[string]string),
	}
}

// Resolve records pkgPath and returns the collision-free Go package identifier
// assigned to it in this file.
func (r *ImportResolver) Resolve(pkgPath, preferredAlias string) string {
	if pkgPath == "" {
		return sanitizeIdent(preferredAlias)
	}
	if alias, ok := r.byPath[pkgPath]; ok {
		return alias
	}
	base := sanitizeIdent(preferredAlias)
	if base == "" {
		base = fallbackPackageName(pkgPath)
	}
	alias := base
	for i := 2; ; i++ {
		if _, exists := r.used[alias]; !exists {
			break
		}
		alias = fmt.Sprintf("%s%d", base, i)
	}
	r.used[alias] = struct{}{}
	r.byPath[pkgPath] = alias
	r.imports = append(r.imports, Import{Path: pkgPath, Alias: alias})
	return alias
}

// AliasFor returns the alias already assigned to pkgPath, or fallback if
// pkgPath was not registered.
func (r *ImportResolver) AliasFor(pkgPath, fallback string) string {
	if r != nil && pkgPath != "" {
		if alias, ok := r.byPath[pkgPath]; ok {
			return alias
		}
	}
	return fallback
}

// Imports returns the resolved imports in registration order.
func (r *ImportResolver) Imports() []Import {
	out := make([]Import, len(r.imports))
	copy(out, r.imports)
	return out
}

// DeduplicateImports removes duplicate import entries, disambiguates colliding
// package identifiers across distinct paths, and sorts them for consistent output.
func DeduplicateImports(extraImports []Import) []Import {
	byPath := make(map[string]Import, len(extraImports))
	paths := make([]string, 0, len(extraImports))
	for _, item := range extraImports {
		if item.Path == "" {
			continue
		}
		if existing, ok := byPath[item.Path]; ok {
			if existing.Alias == "" && item.Alias != "" {
				byPath[item.Path] = item
			}
			continue
		}
		byPath[item.Path] = item
		paths = append(paths, item.Path)
	}
	sort.Strings(paths)

	usedIdents := make(map[string]struct{}, len(paths))
	result := make([]Import, 0, len(paths))
	for _, p := range paths {
		item := byPath[p]
		ident := sanitizeIdent(item.Alias)
		if ident == "" {
			ident = fallbackPackageName(item.Path)
		}
		if _, used := usedIdents[ident]; used {
			base := ident
			for i := 2; ; i++ {
				cand := fmt.Sprintf("%s%d", base, i)
				if _, exists := usedIdents[cand]; !exists {
					ident = cand
					break
				}
			}
			item.Alias = ident
		} else if item.Alias != "" && item.Alias != ident {
			item.Alias = ident
		}
		usedIdents[ident] = struct{}{}

		if item.Alias != "" && canElideAlias(item.Path, item.Alias) {
			item.Alias = ""
		}
		result = append(result, item)
	}
	return result
}

func canElideAlias(importPath, alias string) bool {
	base := path.Base(importPath)
	if alias != base || !token.IsIdentifier(alias) {
		return false
	}
	// /v2+ modules commonly declare the parent package name rather than "v2", so
	// an explicit "v2" alias must be kept to avoid binding the wrong identifier.
	return !isMajorVersionSuffix(base)
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
	importPath, symbol, ok := splitFuncFullName(full)
	if !ok {
		return FuncInfo{FullName: full, Name: full}
	}

	pkg := packageNameFromSource(rf, pc)
	if pkg == "" {
		pkg = fallbackPackageName(importPath)
	}

	return FuncInfo{
		ImportPath: importPath,
		Package:    pkg,
		Name:       symbol,
		FullName:   full,
	}
}

func splitFuncFullName(full string) (importPath, symbol string, ok bool) {
	head := full
	if idx := strings.IndexAny(head, "[("); idx != -1 {
		head = head[:idx]
	}
	lastSlash := strings.LastIndex(head, "/")
	searchFrom := 0
	if lastSlash != -1 {
		searchFrom = lastSlash + 1
	}
	dotRel := strings.Index(head[searchFrom:], ".")
	if dotRel == -1 {
		return "", "", false
	}
	dot := searchFrom + dotRel
	return unescapeSymbolPath(full[:dot]), full[dot+1:], true
}

func packageNameFromSource(rf *runtime.Func, pc uintptr) string {
	file, _ := rf.FileLine(pc)
	if file == "" {
		return ""
	}
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.PackageClauseOnly)
	if err != nil || f.Name == nil {
		return ""
	}
	return f.Name.Name
}

func fallbackPackageName(importPath string) string {
	seg := path.Base(importPath)
	if isMajorVersionSuffix(seg) {
		dir := path.Dir(importPath)
		if dir != "." && dir != "/" {
			seg = path.Base(dir)
		}
	}
	if i := strings.LastIndex(seg, ".v"); i > 0 && isDigits(seg[i+2:]) {
		seg = seg[:i]
	}
	return sanitizeIdent(seg)
}

func isMajorVersionSuffix(seg string) bool {
	if len(seg) < 2 || seg[0] != 'v' || !isDigits(seg[1:]) {
		return false
	}
	n, err := strconv.Atoi(seg[1:])
	return err == nil && n >= 2
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func sanitizeIdent(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range s {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		isDigit := r >= '0' && r <= '9'
		switch {
		case isLetter || (i > 0 && isDigit):
			b.WriteRune(r)
		case i == 0 && isDigit:
			b.WriteByte('_')
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	ident := b.String()
	if ident == "" || ident == "_" || token.Lookup(ident).IsKeyword() {
		return "pkg_" + ident
	}
	return ident
}

// unescapeSymbolPath reverses cmd/internal/objabi.PathToPrefix %xx escaping.
func unescapeSymbolPath(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			if b, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				out = append(out, byte(b))
				i += 3
				continue
			}
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}
