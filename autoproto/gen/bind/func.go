package bind

import (
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/go-sphere/entc-extensions/autoproto/gen/conf"
	"github.com/go-sphere/entc-extensions/autoproto/utils/inspect"
	"github.com/go-sphere/entc-extensions/autoproto/utils/strcase"
)

//go:embed func.tmpl
var genBindFuncTemplate string

// GenBindFunc generates Go code for binding functions based on the provided configuration.
// It creates functions that can convert between source and target types using reflection
// to analyze field mappings and generate appropriate setter calls.
// Returns the generated Go code as a string or an error if generation fails.
func GenBindFunc(action any, conf *conf.EntityConf) (string, error) {
	actionName := inspect.TypeName(action)
	sourceName := inspect.TypeName(conf.Source)
	targetName := inspect.TypeName(conf.Target)
	funcName := strings.Replace(actionName, sourceName, "", 1) + sourceName

	keys, sourceFields := inspect.ExtractPublicFields(conf.Source, strcase.ToSnake)
	_, targetFields := inspect.ExtractPublicFields(conf.Target, strcase.ToSnake)
	_, actionMethods := inspect.ExtractPublicMethods(action, strcase.ToSnake)

	context := bindContext{
		SourcePkgName: conf.SourcePkgName,
		TargetPkgName: conf.TargetPkgName,

		ActionName: actionName,
		SourceName: sourceName,
		TargetName: targetName,
		FuncName:   funcName,
		Fields:     make([]fieldContext, 0),
	}

	ignoreFields := make(map[string]bool, len(conf.IgnoreFields))
	for _, field := range conf.IgnoreFields {
		ignoreFields[strings.ToLower(field)] = true
	}
	table := inspect.TypeName(conf.Source)

	for _, n := range keys {
		if ignoreFields[n] {
			continue
		}
		sourceField, ok := sourceFields[n] // ent.Example
		if !ok {
			continue
		}
		targetField, ok := targetFields[n] // entpb.Example
		if !ok {
			continue
		}

		setter, hasSetter := actionMethods[strcase.ToSnake(fmt.Sprintf("Set%s", sourceField.Name))]
		if !hasSetter {
			continue
		}
		settNillable, hasSettNillable := actionMethods[strcase.ToSnake(fmt.Sprintf("SetNillable%s", sourceField.Name))]
		clearOnNil, hasClearOnNil := actionMethods[strcase.ToSnake(fmt.Sprintf("Clear%s", sourceField.Name))]
		targetFieldIsPtr := targetField.Type.Kind() == reflect.Ptr

		field := fieldContext{
			FieldKeyPath: fmt.Sprintf("%s.Field%s", strings.ToLower(table), sourceField.Name),

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
		context.Fields = append(context.Fields, field)
	}

	parse, err := template.New("bind").Funcs(template.FuncMap{
		"GenZeroCheck":    inspect.GenerateZeroCheckExpr,
		"GenNotZeroCheck": inspect.GenerateNonZeroCheckExpr,
		"ToSnakeCase":     strcase.ToSnake,
	}).Parse(genBindFuncTemplate)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	err = parse.Execute(&builder, context)
	if err != nil {
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

	Fields []fieldContext
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
}
