package bind

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
	"sync"
	"text/template"

	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/entcrud/internal/inspect"
	"github.com/go-sphere/entc-extensions/entcrud/internal/strcase"
)

//go:embed func.tmpl
var genBindFuncTemplate string

var (
	bindTemplateOnce sync.Once
	bindTemplate     *template.Template
	bindTemplateErr  error
)

func getBindTemplate() (*template.Template, error) {
	bindTemplateOnce.Do(func() {
		bindTemplate, bindTemplateErr = template.New("bind").Funcs(template.FuncMap{
			"GenZeroCheck":          inspect.GenerateZeroCheckExpr,
			"GenNotZeroCheck":       inspect.GenerateNonZeroCheckExpr,
			"GenTypeConversionExpr": inspect.GenerateTypeConversionExpr,
			"ToSnakeCase":           strcase.ToSnake,
		}).Parse(genBindFuncTemplate)
	})
	return bindTemplate, bindTemplateErr
}

// GenBindFunc generates Go code for binding functions.
func GenBindFunc(action any, entityConf *conf.EntityConf, customConverters map[string]any, strictTypeCheck bool) (string, error) {
	actionName := inspect.TypeName(action)
	sourceName := inspect.TypeName(entityConf.Source)
	targetName := inspect.TypeName(entityConf.Target)
	funcName := bindFuncName(actionName, sourceName)

	keys, sourceFields := inspect.ExtractPublicFields(entityConf.Source, strcase.ToSnake)
	_, targetFields := inspect.ExtractPublicFields(entityConf.Target, strcase.ToSnake)
	_, actionMethods := inspect.ExtractPublicMethods(action, strcase.ToSnake)

	ignoreFields := normalizeIgnoredFields(entityConf.IgnoreFields)
	table := strings.ToLower(sourceName)
	fields, mismatches := buildFieldContexts(
		keys,
		sourceFields,
		targetFields,
		actionMethods,
		table,
		ignoreFields,
		customConverters,
		strictTypeCheck,
		sourceName,
	)
	if len(mismatches) > 0 {
		return "", &conf.TypeMismatchListError{Items: mismatches}
	}

	context := bindContext{
		SourcePkgName: "ent",                                         // Source uses ent package (e.g., ent.ExampleCreate)
		TargetPkgName: inspect.ExtractPackageName(entityConf.Target), // Target uses entpb package

		ActionName: actionName,
		SourceName: sourceName,
		TargetName: targetName,
		FuncName:   funcName,
		Fields:     fields,
	}

	parse, err := getBindTemplate()
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	if err := parse.Execute(&builder, context); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func normalizeIgnoredFields(fields []string) map[string]struct{} {
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		result[strings.ToLower(field)] = struct{}{}
	}
	return result
}

// bindFuncName derives the generated function name from an action type and the
// source entity type it operates on. ent generates action types named after
// the entity plus a verb suffix (UserCreate, UserUpdateOne), so the entity name
// is stripped from the front of the action name and re-appended at the end to
// produce CreateUser / UpdateOneUser.
//
// Unlike a blanket substring removal, the entity prefix is only stripped when
// the action name actually starts with it. This prevents corrupting names when
// the entity name appears in the middle of an unrelated action identifier.
func bindFuncName(actionName, sourceName string) string {
	if strings.HasPrefix(actionName, sourceName) {
		return strings.TrimPrefix(actionName, sourceName) + sourceName
	}
	return actionName
}

func buildFieldContexts(
	keys []string,
	sourceFields map[string]reflect.StructField,
	targetFields map[string]reflect.StructField,
	actionMethods map[string]reflect.Method,
	table string,
	ignoreFields map[string]struct{},
	customConverters map[string]any,
	strictTypeCheck bool,
	entityName string,
) ([]fieldContext, []conf.TypeMismatchError) {
	fields := make([]fieldContext, 0, len(keys))
	mismatches := make([]conf.TypeMismatchError, 0)
	for _, key := range keys {
		if _, ignored := ignoreFields[key]; ignored {
			continue
		}
		sourceField, ok := sourceFields[key]
		if !ok {
			continue
		}
		targetField, ok := targetFields[key]
		if !ok {
			continue
		}

		field, ok, mismatch := buildFieldContext(key, sourceField, targetField, actionMethods, table, customConverters, entityName)
		if !ok {
			continue
		}
		if mismatch != nil {
			if strictTypeCheck {
				mismatches = append(mismatches, *mismatch)
			}
			continue
		}
		fields = append(fields, field)
	}
	return fields, mismatches
}

func buildFieldContext(
	key string,
	sourceField reflect.StructField,
	targetField reflect.StructField,
	actionMethods map[string]reflect.Method,
	table string,
	customConverters map[string]any,
	entityName string,
) (fieldContext, bool, *conf.TypeMismatchError) {
	setter, hasSetter := actionMethods[strcase.ToSnake("Set"+sourceField.Name)]
	if !hasSetter {
		return fieldContext{}, false, nil
	}
	setterArgType, ok := setterArgumentType(setter)
	if !ok {
		return fieldContext{}, true, &conf.TypeMismatchError{
			Entity:     entityName,
			Field:      sourceField.Name,
			SourceType: setter.Type.String(),
			TargetType: targetField.Type.String(),
			Suggestion: "setter must accept exactly one field value",
		}
	}

	settNillable, hasSettNillable := actionMethods[strcase.ToSnake("SetNillable"+sourceField.Name)]
	clearOnNil, hasClearOnNil := actionMethods[strcase.ToSnake("Clear"+sourceField.Name)]
	targetFieldIsPtr := targetField.Type.Kind() == reflect.Pointer

	field := fieldContext{
		FieldKeyPath: table + ".Field" + sourceField.Name,

		TargetField:   targetField,
		SourceField:   sourceField,
		SetterArgType: setterArgType,

		SetterFuncName:       setter.Name,
		SettNillableFuncName: settNillable.Name,
		ClearOnNilFuncName:   clearOnNil.Name,

		CanSettNillable:  hasSettNillable,
		CanClearOnNil:    hasClearOnNil,
		TargetFieldIsPtr: targetFieldIsPtr,
	}

	if targetFieldIsPtr {
		field.TargetSourceIsSomeType = targetField.Type.Elem().AssignableTo(setterArgType)
	} else {
		field.TargetSourceIsSomeType = targetField.Type.AssignableTo(setterArgType)
	}

	if converter, ok := customConverters[key]; ok {
		info, mismatch := validateCustomConverter(converter, targetField.Type, setterArgType, entityName, sourceField.Name)
		if mismatch != nil {
			return field, true, mismatch
		}
		field.HasCustomConverter = true
		field.CustomConverter = info
		return field, true, nil
	}

	targetValueType := targetField.Type
	if targetFieldIsPtr {
		targetValueType = targetValueType.Elem()
	}
	if !targetValueType.AssignableTo(setterArgType) && !targetValueType.ConvertibleTo(setterArgType) {
		return field, true, &conf.TypeMismatchError{
			Entity:     entityName,
			Field:      sourceField.Name,
			SourceType: setterArgType.String(),
			TargetType: targetField.Type.String(),
			Suggestion: fmt.Sprintf("add conf.WithCustomFieldConverter(%s.Field%s, <converter>)", strings.ToLower(entityName), sourceField.Name),
		}
	}

	return field, true, nil
}

func setterArgumentType(method reflect.Method) (reflect.Type, bool) {
	if method.Type.NumIn() != 2 {
		return nil, false
	}
	return method.Type.In(1), true
}

func validateCustomConverter(
	converter any,
	targetType reflect.Type,
	setterArgType reflect.Type,
	entityName string,
	fieldName string,
) (inspect.FuncInfo, *conf.TypeMismatchError) {
	converterType := reflect.TypeOf(converter)
	info := inspect.GetFuncInfo(converter)
	invalid := converterType == nil || converterType.Kind() != reflect.Func ||
		converterType.NumIn() != 1 || converterType.NumOut() != 1 ||
		info.ImportPath == "" || !token.IsIdentifier(info.Name) || !ast.IsExported(info.Name)
	valueType := targetType
	if valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if !invalid {
		invalid = !valueType.AssignableTo(converterType.In(0)) || !converterType.Out(0).AssignableTo(setterArgType)
	}
	if invalid {
		return inspect.FuncInfo{}, &conf.TypeMismatchError{
			Entity:     entityName,
			Field:      fieldName,
			SourceType: setterArgType.String(),
			TargetType: targetType.String(),
			Suggestion: "custom converter must be an exported top-level func with one input matching the protobuf value and one output assignable to the Ent setter",
		}
	}
	return info, nil
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

	TargetField   reflect.StructField
	SourceField   reflect.StructField
	SetterArgType reflect.Type

	SetterFuncName       string
	SettNillableFuncName string
	ClearOnNilFuncName   string

	CanSettNillable bool
	CanClearOnNil   bool

	TargetFieldIsPtr       bool
	TargetSourceIsSomeType bool

	HasCustomConverter bool
	CustomConverter    inspect.FuncInfo
}
