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
		conf.WithCustomFieldConverter("birthday", func(v int64) time.Time { return time.Unix(v, 0) }),
	)

	code, err := GenBindFunc(createAction{}, entity, entity.CustomFieldConverters, true)
	if err != nil {
		t.Fatalf("GenBindFunc failed: %v", err)
	}
	if !strings.Contains(code, "SetBirthday") {
		t.Fatalf("expected generated code to contain SetBirthday, got:\n%s", code)
	}
}
