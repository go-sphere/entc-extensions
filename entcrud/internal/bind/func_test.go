package bind

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/entc-extensions/entcrud/conf"
)

type sourceEntity struct {
	Birthday time.Time
}

type targetMessage struct {
	Birthday int64
}

type createAction struct{}

func (createAction) SetBirthday(time.Time) {}

func BirthdayFromUnix(v int64) time.Time {
	return time.Unix(v, 0)
}

func TestGenBindFunc_StrictTypeCheckReturnsStructuredError(t *testing.T) {
	entity := conf.NewEntity(sourceEntity{}, targetMessage{}, []any{createAction{}})

	_, err := GenBindFunc(createAction{}, entity, nil, true)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}

	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected *conf.TypeMismatchListError, got %T (%v)", err, err)
	}
	if len(mismatch.Items) != 1 {
		t.Fatalf("expected 1 mismatch item, got %d", len(mismatch.Items))
	}
	item := mismatch.Items[0]
	if item.Entity != "sourceEntity" {
		t.Fatalf("entity = %q, want sourceEntity", item.Entity)
	}
	if item.Field != "Birthday" {
		t.Fatalf("field = %q, want Birthday", item.Field)
	}
	if item.SourceType != "time.Time" || item.TargetType != "int64" {
		t.Fatalf("types = (%s <- %s), want (time.Time <- int64)", item.SourceType, item.TargetType)
	}
}

func TestGenBindFunc_NonStrictSkipsIncompatibleField(t *testing.T) {
	entity := conf.NewEntity(sourceEntity{}, targetMessage{}, []any{createAction{}})

	code, err := GenBindFunc(createAction{}, entity, nil, false)
	if err != nil {
		t.Fatalf("GenBindFunc failed: %v", err)
	}
	if strings.Contains(code, "SetBirthday") {
		t.Fatalf("expected incompatible field to be skipped, got code:\n%s", code)
	}
}

func TestGenBindFunc_CustomConverterOverridesMismatch(t *testing.T) {
	entity := conf.NewEntity(
		sourceEntity{},
		targetMessage{},
		[]any{createAction{}},
		conf.WithCustomFieldConverter("birthday", BirthdayFromUnix),
	)

	code, err := GenBindFunc(createAction{}, entity, entity.CustomFieldConverters, true)
	if err != nil {
		t.Fatalf("GenBindFunc failed: %v", err)
	}
	if !strings.Contains(code, "SetBirthday") {
		t.Fatalf("expected generated code to contain SetBirthday, got:\n%s", code)
	}
}

func TestGenBindFunc_RejectsAnonymousCustomConverter(t *testing.T) {
	entity := conf.NewEntity(
		sourceEntity{},
		targetMessage{},
		[]any{createAction{}},
		conf.WithCustomFieldConverter("birthday", func(v int64) time.Time { return time.Unix(v, 0) }),
	)

	_, err := GenBindFunc(createAction{}, entity, entity.CustomFieldConverters, true)
	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected TypeMismatchListError, got %T (%v)", err, err)
	}
}

type pointerTargetMessage struct {
	Birthday *string
}

func TestGenBindFunc_StrictTypeCheckRejectsIncompatiblePointer(t *testing.T) {
	entity := conf.NewEntity(sourceEntity{}, pointerTargetMessage{}, []any{createAction{}})

	_, err := GenBindFunc(createAction{}, entity, nil, true)
	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected TypeMismatchListError, got %T (%v)", err, err)
	}
}

type nillableSourceEntity struct {
	Nickname *string
}

type optionalTargetMessage struct {
	Nickname *string
}

type nillableCreateAction struct{}

func (nillableCreateAction) SetNickname(string)          {}
func (nillableCreateAction) SetNillableNickname(*string) {}
func (nillableCreateAction) ClearNickname()              {}

func TestGenBindFunc_AcceptsMatchingOptionalPointer(t *testing.T) {
	entity := conf.NewEntity(nillableSourceEntity{}, optionalTargetMessage{}, []any{nillableCreateAction{}})

	code, err := GenBindFunc(nillableCreateAction{}, entity, nil, true)
	if err != nil {
		t.Fatalf("GenBindFunc failed: %v", err)
	}
	if !strings.Contains(code, "SetNillableNickname(target.Nickname)") {
		t.Fatalf("generated code does not preserve optional pointer:\n%s", code)
	}
}

type numericSourceEntity struct {
	Age int
}

type optionalNumericTargetMessage struct {
	Age *int32
}

type numericCreateAction struct{}

func (numericCreateAction) SetAge(int) {}

func TestGenBindFunc_DereferencesPointerBeforeNumericConversion(t *testing.T) {
	entity := conf.NewEntity(numericSourceEntity{}, optionalNumericTargetMessage{}, []any{numericCreateAction{}})

	code, err := GenBindFunc(numericCreateAction{}, entity, nil, true)
	if err != nil {
		t.Fatalf("GenBindFunc failed: %v", err)
	}
	if !strings.Contains(code, "SetAge(int(*target.Age))") {
		t.Fatalf("generated code does not dereference the pointer before conversion:\n%s", code)
	}
}

func TestBindFuncName(t *testing.T) {
	cases := []struct {
		action, source, want string
	}{
		{"UserCreate", "User", "CreateUser"},
		{"UserUpdateOne", "User", "UpdateOneUser"},
		// Entity name appearing in the middle must NOT be stripped.
		{"SomeUserCreate", "User", "SomeUserCreate"},
		// Action not prefixed with the source name stays untouched.
		{"Upsert", "User", "Upsert"},
	}
	for _, c := range cases {
		if got := bindFuncName(c.action, c.source); got != c.want {
			t.Errorf("bindFuncName(%q,%q) = %q, want %q", c.action, c.source, got, c.want)
		}
	}
}

func TestCreateOptionsFileRewritesOnlyPackageDeclaration(t *testing.T) {
	got := CreateOptionsFile("entbind")
	if !strings.HasPrefix(got, "// Code generated by Sphere. DO NOT EDIT.\npackage entbind\n") {
		t.Fatalf("generated file does not start with rewritten package declaration:\n%s", got)
	}
	if strings.Contains(got, "package bind") {
		t.Fatalf("generated file still contains the original package declaration:\n%s", got)
	}
}

type noFieldsSource struct{ Name string }

type noFieldsTarget struct{}

type noFieldsAction struct{}

func (noFieldsAction) SetName(string) {}

// TestGenBindFunc_NoBindableFieldsOmitsUnusedOption ensures an entity whose
// target shares no field with the source does not emit an unused `option`
// variable (which would fail to compile).
func TestGenBindFunc_NoBindableFieldsOmitsUnusedOption(t *testing.T) {
	entity := conf.NewEntity(noFieldsSource{}, noFieldsTarget{}, []any{noFieldsAction{}})
	code, err := GenBindFunc(noFieldsAction{}, entity, nil, true)
	if err != nil {
		t.Fatalf("GenBindFunc failed: %v", err)
	}
	if strings.Contains(code, "option := NewBindOptions") {
		t.Fatalf("generated code declares unused option variable:\n%s", code)
	}
	if !strings.Contains(code, "return source") {
		t.Fatalf("generated code should still return source:\n%s", code)
	}
}
