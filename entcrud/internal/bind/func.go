package bind

import (
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/entcrud/internal/inspect"
	"github.com/go-sphere/entc-extensions/entcrud/internal/strcase"
)

//go:embed func.tmpl
var genBindFuncTemplate string

// GenBindFunc generates Go code for binding functions.
func GenBindFunc(action any, conf *conf.EntityConf, customConverters map[string]any) (string, error) {
	actionName := inspect.TypeName(action)
	sourceName := inspect.TypeName(conf.Source)
	targetName := inspect.TypeName(conf.Target)
	funcName := strings.Replace(actionName, sourceName, "", 1) + sourceName

	keys, sourceFields := inspect.ExtractPublicFields(conf.Source, strcase.ToSnake)
	_, targetFields := inspect.ExtractPublicFields(conf.Target, strcase.ToSnake)
	_, actionMethods := inspect.ExtractPublicMethods(action, strcase.ToSnake)

	ignoreFields := make(map[string]bool, len(conf.IgnoreFields))
	for _, field := range conf.IgnoreFields {
		ignoreFields[strings.ToLower(field)] = true
	}
	table := strings.ToLower(inspect.TypeName(conf.Source))

	fields := make([]fieldContext, 0, len(keys))
	for _, n := range keys {
		if ignoreFields[n] {
			continue
		}
		sourceField, ok := sourceFields[n]
		if !ok {
			continue
		}
		targetField, ok := targetFields[n]
		if !ok {
			continue
		}

		setter, hasSetter := actionMethods[strcase.ToSnake(fmt.Sprintf("Set%s", sourceField.Name))]
		if !hasSetter {
			continue
		}

		settNillable, hasSettNillable := actionMethods[strcase.ToSnake(fmt.Sprintf("SetNillable%s", sourceField.Name))]
		clearOnNil, hasClearOnNil := actionMethods[strcase.ToSnake(fmt.Sprintf("Clear%s", sourceField.Name))]
		targetFieldIsPtr := targetField.Type.Kind() == reflect.Pointer

		field := fieldContext{
			FieldKeyPath: fmt.Sprintf("%s.Field%s", table, sourceField.Name),

			TargetField: targetField,
			SourceField: sourceField,

			SetterFuncName:       setter.Name,
			SettNillableFuncName: settNillable.Name,
			ClearOnNilFuncName:   clearOnNil.Name,

			CanSettNillable:        hasSettNillable,
			CanClearOnNil:          hasClearOnNil,
			TargetFieldIsPtr:       targetFieldIsPtr,
			TargetSourceIsSomeType: false,
		}

		if targetFieldIsPtr {
			elem := targetField.Type.Elem()
			field.TargetSourceIsSomeType = elem.Kind() == sourceField.Type.Kind() && elem.String() == sourceField.Type.String()
		} else {
			field.TargetSourceIsSomeType = targetField.Type.Kind() == sourceField.Type.Kind() && targetField.Type.String() == sourceField.Type.String()
		}

		// Check if types are compatible - skip if not
		if !field.TargetSourceIsSomeType && !targetFieldIsPtr {
			// Check for incompatible types (e.g., time.Time vs int64)
			sourceType := sourceField.Type.String()
			targetType := targetField.Type.String()
			if (sourceType == "time.Time" && targetType == "int64") ||
				(sourceType == "int64" && targetType == "time.Time") {
				field.SkipField = true
			}
		}

		// Check for custom converter
		if customConverters != nil {
			if converter, ok := customConverters[n]; ok {
				field.HasCustomConverter = true
				field.CustomConverter = inspect.GetFuncInfo(converter)
				field.SkipField = false // Custom converter can handle incompatible types
			}
		}

		fields = append(fields, field)
	}

	context := bindContext{
		SourcePkgName: "ent",                                   // Source uses ent package (e.g., ent.ExampleCreate)
		TargetPkgName: inspect.ExtractPackageName(conf.Target), // Target uses entpb package

		ActionName: actionName,
		SourceName: sourceName,
		TargetName: targetName,
		FuncName:   funcName,
		Fields:     fields,
	}

	parse, err := template.New("bind").Funcs(template.FuncMap{
		"GenZeroCheck":          inspect.GenerateZeroCheckExpr,
		"GenNotZeroCheck":       inspect.GenerateNonZeroCheckExpr,
		"GenTypeConversionExpr": inspect.GenerateTypeConversionExpr,
		"ToSnakeCase":           strcase.ToSnake,
	}).Parse(genBindFuncTemplate)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	if err := parse.Execute(&builder, context); err != nil {
		return "", err
	}
	return builder.String(), nil
}

type bindContext struct {
	SourcePkgName string
	TargetPkgName string

	ActionName string
	SourceName string
	TargetName string
	FuncName   string
	Fields     []fieldContext
}

type fieldContext struct {
	FieldKeyPath string

	TargetField reflect.StructField
	SourceField reflect.StructField

	SetterFuncName       string
	SettNillableFuncName string
	ClearOnNilFuncName   string

	CanSettNillable bool
	CanClearOnNil   bool

	TargetFieldIsPtr       bool
	TargetSourceIsSomeType bool
	SkipField              bool // Skip this field if types are incompatible

	HasCustomConverter bool
	CustomConverter    inspect.FuncInfo
}
