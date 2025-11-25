package mapper

import (
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/go-sphere/entc-extensions/autoproto/utils/inspect"
	"github.com/iancoleman/strcase"
)

//go:embed mapper.tmpl
var mapperTemplate string

// GenFuncConf configures the generation of mapper functions for a single entity pair.
type GenFuncConf struct {
	source        any
	target        any
	IgnoreFields  []string
	SourcePkgName string
	TargetPkgName string
}

// NewGenFuncConf creates a new configuration for generating mapper functions.
func NewGenFuncConf(source, target any) *GenFuncConf {
	return &GenFuncConf{
		source:        source,
		target:        target,
		SourcePkgName: inspect.ExtractPackageName(source),
		TargetPkgName: inspect.ExtractPackageName(target),
	}
}

// WithSourcePkgName overrides the detected source package name.
func (c *GenFuncConf) WithSourcePkgName(pkg string) *GenFuncConf {
	if pkg != "" {
		c.SourcePkgName = pkg
	}
	return c
}

// WithTargetPkgName overrides the detected target package name.
func (c *GenFuncConf) WithTargetPkgName(pkg string) *GenFuncConf {
	if pkg != "" {
		c.TargetPkgName = pkg
	}
	return c
}

// WithIgnoreFields sets the fields that should be skipped during generation.
func (c *GenFuncConf) WithIgnoreFields(fields ...string) *GenFuncConf {
	c.IgnoreFields = append([]string(nil), fields...)
	return c
}

// GenFunc renders the mapper functions for the provided configuration.
func GenFunc(conf *GenFuncConf) (string, error) {
	sourceName := inspect.TypeName(conf.source)
	targetName := inspect.TypeName(conf.target)

	keys, sourceFields := inspect.ExtractPublicFields(conf.source, strcase.ToSnake)
	_, targetFields := inspect.ExtractPublicFields(conf.target, strcase.ToSnake)

	ignore := normaliseIgnoreFields(conf.IgnoreFields)

	ctx := mapperContext{
		SourcePkgName: conf.SourcePkgName,
		TargetPkgName: conf.TargetPkgName,
		SourceName:    sourceName,
		TargetName:    targetName,
		FuncName:      fmt.Sprintf("ToProto%s", sourceName),
		ListFuncName:  fmt.Sprintf("ToProto%sList", sourceName),
	}

	for _, key := range keys {
		lower := strings.ToLower(key)
		if _, skip := ignore[lower]; skip {
			continue
		}

		sField, ok := sourceFields[key]
		if !ok {
			continue
		}
		tField, ok := targetFields[key]
		if !ok {
			continue
		}

		fieldCtx, ok := buildFieldContext(sField, tField)
		if !ok {
			continue
		}
		ctx.Fields = append(ctx.Fields, *fieldCtx)
	}

	tmpl, err := template.New("mapper").Parse(mapperTemplate)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	if err := tmpl.ExecuteTemplate(&builder, "mapper", ctx); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func normaliseIgnoreFields(fields []string) map[string]struct{} {
	ignore := make(map[string]struct{}, len(fields)*2)
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		ignore[strings.ToLower(field)] = struct{}{}
		parts := strings.Split(field, ".")
		base := parts[len(parts)-1]
		ignore[strings.ToLower(strcase.ToSnake(base))] = struct{}{}
	}
	return ignore
}

type mapperContext struct {
	SourcePkgName string
	TargetPkgName string
	SourceName    string
	TargetName    string
	FuncName      string
	ListFuncName  string
	Fields        []fieldContext
}

type fieldContext struct {
	TargetField string
	ValueExpr   string
	GuardExpr   string
}

func buildFieldContext(sourceField, targetField reflect.StructField) (*fieldContext, bool) {
	sExpr := fmt.Sprintf("source.%s", sourceField.Name)

	if expr, ok := buildAssignmentExpr(sExpr, sourceField.Type, targetField.Type); ok {
		return &fieldContext{
			TargetField: targetField.Name,
			ValueExpr:   expr,
		}, true
	}

	if sourceField.Type.Kind() == reflect.Ptr {
		guard := fmt.Sprintf("%s != nil", sExpr)
		if expr, ok := buildAssignmentExpr("*"+sExpr, sourceField.Type.Elem(), targetField.Type); ok {
			return &fieldContext{
				TargetField: targetField.Name,
				ValueExpr:   expr,
				GuardExpr:   guard,
			}, true
		}
	}

	if targetField.Type.Kind() == reflect.Ptr && sourceField.Type.Kind() != reflect.Ptr {
		if expr, ok := buildAssignmentExpr(sExpr, sourceField.Type, targetField.Type.Elem()); ok {
			typeName := targetField.Type.Elem().String()
			wrapped := fmt.Sprintf("func(v %s) *%s { return &v }(%s)", typeName, typeName, expr)
			return &fieldContext{
				TargetField: targetField.Name,
				ValueExpr:   wrapped,
			}, true
		}
	}

	return nil, false
}

func buildAssignmentExpr(sourceExpr string, sourceType, targetType reflect.Type) (string, bool) {
	if sourceType.AssignableTo(targetType) {
		return sourceExpr, true
	}
	if isSimpleConvertible(sourceType, targetType) {
		return fmt.Sprintf("%s(%s)", targetType.String(), sourceExpr), true
	}
	return "", false
}

func isSimpleConvertible(sourceType, targetType reflect.Type) bool {
	if isIntKind(sourceType.Kind()) && isIntKind(targetType.Kind()) {
		return true
	}
	if isUintKind(sourceType.Kind()) && isUintKind(targetType.Kind()) {
		return true
	}
	if isFloatKind(sourceType.Kind()) && isFloatKind(targetType.Kind()) {
		return true
	}
	if sourceType.Kind() == reflect.String && targetType.Kind() == reflect.String {
		return true
	}
	if sourceType.Kind() == reflect.Bool && targetType.Kind() == reflect.Bool {
		return true
	}
	return false
}

func isIntKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func isUintKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func isFloatKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}
