package bind

import (
	"errors"
	"go/format"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/entc-extensions/entcrud/conf"
	"github.com/go-sphere/entc-extensions/entcrud/internal/inspect"
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

type entStatus string

type pbStatus int32

type enumSourceEntity struct {
	Status entStatus
}

type enumTargetMessage struct {
	Status pbStatus
}

type enumCreateAction struct{}

func (enumCreateAction) SetStatus(entStatus) {}

// Go accepts the int → string conversion, but it yields the code point's rune
// ("\x01"), so a proto enum bound to an ent string enum must be a mismatch.
func TestGenBindFunc_StrictTypeCheckRejectsIntegerToString(t *testing.T) {
	entity := conf.NewEntity(enumSourceEntity{}, enumTargetMessage{}, []any{enumCreateAction{}})

	_, err := GenBindFunc(enumCreateAction{}, entity, nil, true)
	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected TypeMismatchListError, got %T (%v)", err, err)
	}
	if got := mismatch.Items[0].Field; got != "Status" {
		t.Fatalf("field = %q, want Status", got)
	}
}

type metaPayload struct{ Note string }

type metaSourceEntity struct {
	Meta *metaPayload
}

type metaTargetMessage struct {
	Meta *metaPayload
}

type metaCreateAction struct{}

func (metaCreateAction) SetMeta(*metaPayload) {}

type metaUpdateAction struct{}

func (metaUpdateAction) SetMeta(*metaPayload) {}
func (metaUpdateAction) ClearMeta()           {}

func TestGenBindFunc_SingularPointerFieldPreservesPointer(t *testing.T) {
	createEntity := conf.NewEntity(metaSourceEntity{}, metaTargetMessage{}, []any{metaCreateAction{}})
	createCode, err := GenBindFunc(metaCreateAction{}, createEntity, nil, true)
	if err != nil {
		t.Fatalf("GenBindFunc create failed: %v", err)
	}
	if strings.Contains(createCode, "*target.Meta") {
		t.Fatalf("singular pointer field should not be dereferenced:\n%s", createCode)
	}
	if !strings.Contains(createCode, "source.SetMeta(target.Meta)") {
		t.Fatalf("expected direct SetMeta(target.Meta):\n%s", createCode)
	}
	if !strings.Contains(createCode, "option.CanSetZero(") || !strings.Contains(createCode, "target.Meta != nil") {
		t.Fatalf("expected CanSetZero check on singular pointer field:\n%s", createCode)
	}

	updateEntity := conf.NewEntity(metaSourceEntity{}, metaTargetMessage{}, []any{metaUpdateAction{}})
	updateCode, err := GenBindFunc(metaUpdateAction{}, updateEntity, nil, true)
	if err != nil {
		t.Fatalf("GenBindFunc update failed: %v", err)
	}
	if !strings.Contains(updateCode, "target.Meta == nil && option.ClearOnNil(") || !strings.Contains(updateCode, "source.ClearMeta()") {
		t.Fatalf("expected ClearOnNil handling on singular pointer field:\n%s", updateCode)
	}
	if !strings.Contains(updateCode, "source.SetMeta(target.Meta)") {
		t.Fatalf("expected direct SetMeta(target.Meta) in update:\n%s", updateCode)
	}
}

type customOptEntity struct {
	Birthday         time.Time
	OptionalBirthday *time.Time
	Meta             *metaPayload
}

type customOptTarget struct {
	Birthday         int64
	OptionalBirthday *int64
	Meta             *metaPayload
}

type customOptCreate struct{}

func (customOptCreate) SetBirthday(time.Time)         {}
func (customOptCreate) SetOptionalBirthday(time.Time) {}

type customOptUpdate struct{}

func (customOptUpdate) SetBirthday(time.Time)                  {}
func (customOptUpdate) SetNillableBirthday(*time.Time)         {}
func (customOptUpdate) SetOptionalBirthday(time.Time)          {}
func (customOptUpdate) SetNillableOptionalBirthday(*time.Time) {}
func (customOptUpdate) ClearOptionalBirthday()                 {}
func (customOptUpdate) SetMeta(*metaPayload)                   {}
func (customOptUpdate) ClearMeta()                             {}

func MetaPtrPassThrough(m *metaPayload) *metaPayload { return m }

func TestGenBindFunc_CustomConverterHonorsCanSetZeroAndClearOnNil(t *testing.T) {
	entity := conf.NewEntity(
		customOptEntity{},
		customOptTarget{},
		[]any{customOptCreate{}, customOptUpdate{}},
		conf.WithCustomFieldConverter("birthday", BirthdayFromUnix),
		conf.WithCustomFieldConverter("optional_birthday", BirthdayFromUnix),
		conf.WithCustomFieldConverter("meta", MetaPtrPassThrough),
	)

	createCode, err := GenBindFunc(customOptCreate{}, entity, entity.CustomFieldConverters, true)
	if err != nil {
		t.Fatalf("GenBindFunc create failed: %v", err)
	}
	if !strings.Contains(createCode, "if option.CanSetZero(customoptentity.FieldBirthday) || target.Birthday != 0 {") {
		t.Fatalf("non-pointer Create custom converter must honor CanSetZero:\n%s", createCode)
	}

	updateCode, err := GenBindFunc(customOptUpdate{}, entity, entity.CustomFieldConverters, true)
	if err != nil {
		t.Fatalf("GenBindFunc update failed: %v", err)
	}
	if !strings.Contains(updateCode, "if target.OptionalBirthday == nil && option.ClearOnNil(customoptentity.FieldOptionalBirthday)") ||
		!strings.Contains(updateCode, "source.ClearOptionalBirthday()") {
		t.Fatalf("pointer custom converter must honor ClearOnNil:\n%s", updateCode)
	}
	if !strings.Contains(updateCode, "MetaPtrPassThrough(target.Meta)") {
		t.Fatalf("pointer-accepting custom converter must receive target.Meta without dereference:\n%s", updateCode)
	}
}

func TestGenBindFuncWithResolver_UsesResolvedAliases(t *testing.T) {
	entity := conf.NewEntity(
		sourceEntity{},
		targetMessage{},
		[]any{createAction{}},
		conf.WithCustomFieldConverter("birthday", BirthdayFromUnix),
	)
	resolver := inspect.NewImportResolver("source", "target", "options", "option")
	resolver.Resolve("github.com/go-sphere/entc-extensions/entcrud/internal/bind", "bind")
	// Simulate subpackage colliding with an earlier target package of the same name.
	resolver.Resolve("example.com/other/sourceentity", "sourceentity")
	resolver.Resolve("github.com/go-sphere/entc-extensions/entcrud/internal/bind/sourceentity", "sourceentity")

	code, err := GenBindFuncWithResolver(createAction{}, entity, entity.CustomFieldConverters, true, resolver)
	if err != nil {
		t.Fatalf("GenBindFuncWithResolver failed: %v", err)
	}
	if !strings.Contains(code, "sourceentity2.FieldBirthday") {
		t.Fatalf("expected disambiguated subpackage alias sourceentity2, got:\n%s", code)
	}
	if _, err := format.Source([]byte("package out\n" + code)); err != nil {
		t.Fatalf("generated function failed gofmt: %v\n%s", err, code)
	}
}

type entIPV4 string

type pbIPV4 int32

func ConvertPBIPV4(v pbIPV4) entIPV4 {
	return entIPV4(rune('0' + v))
}

func NormalizeS3Key(v string) string {
	return strings.TrimPrefix(v, "/")
}

type taggedAcronymSource struct {
	IPV4    entIPV4 `json:"ip_v4,omitempty"`
	HTTPURL string  `json:"http_url,omitempty"`
	S3Key   string  `json:"s3_key,omitempty"`
}

type taggedAcronymTarget struct {
	IpV4    pbIPV4 `protobuf:"varint,10,opt,name=ip_v4,json=ipV4,proto3" json:"ip_v4,omitempty"`
	HttpUrl string `protobuf:"bytes,11,opt,name=http_url,json=httpUrl,proto3" json:"httpUrl,omitempty"`
	S3Key   string `json:"s3_key,omitempty"`
}

type optionalAcronymTarget struct {
	IpV4    *pbIPV4 `protobuf:"varint,10,opt,name=ip_v4,json=ipV4,proto3,oneof" json:"ip_v4,omitempty"`
	HttpUrl string  `json:"http_url,omitempty"`
	S3Key   string  `json:"s3_key,omitempty"`
}

type untaggedAcronymTarget struct {
	IPV4    pbIPV4
	HTTPURL string
	S3Key   string
}

type taggedAcronymCreate struct{}

func (taggedAcronymCreate) SetIPV4(entIPV4)   {}
func (taggedAcronymCreate) SetHTTPURL(string) {}
func (taggedAcronymCreate) SetS3Key(string)   {}

type taggedAcronymUpdateOne struct{}

func (taggedAcronymUpdateOne) SetIPV4(entIPV4)   {}
func (taggedAcronymUpdateOne) ClearIPV4()        {}
func (taggedAcronymUpdateOne) SetHTTPURL(string) {}
func (taggedAcronymUpdateOne) SetS3Key(string)   {}

func TestGenBindFunc_AcronymAndDigitFieldsViaTags(t *testing.T) {
	// Without a custom converter, IPV4 <-> IpV4 must still be matched via tags and
	// rejected by strict type checking rather than silently skipped.
	unconverted := conf.NewEntity(taggedAcronymSource{}, taggedAcronymTarget{}, []any{taggedAcronymCreate{}})
	_, err := GenBindFunc(taggedAcronymCreate{}, unconverted, nil, true)
	var mismatch *conf.TypeMismatchListError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected TypeMismatchListError for IPV4 <-> IpV4, got %T (%v)", err, err)
	}
	if len(mismatch.Items) != 1 || mismatch.Items[0].Field != "IPV4" {
		t.Fatalf("unexpected mismatch items: %+v", mismatch.Items)
	}

	// Canonical tag keys work for custom converters on IPV4, HTTPURL, and S3Key.
	entity := conf.NewEntity(
		taggedAcronymSource{},
		taggedAcronymTarget{},
		[]any{taggedAcronymCreate{}, taggedAcronymUpdateOne{}},
		conf.WithCustomFieldConverter("ip_v4", ConvertPBIPV4),
		conf.WithCustomFieldConverter("s3_key", NormalizeS3Key),
	)
	code, err := GenBindFunc(taggedAcronymCreate{}, entity, entity.CustomFieldConverters, true)
	if err != nil {
		t.Fatalf("GenBindFunc create failed: %v", err)
	}
	for _, want := range []string{
		"SetIPV4(bind.ConvertPBIPV4(target.IpV4))",
		"SetHTTPURL(target.HttpUrl)",
		"SetS3Key(bind.NormalizeS3Key(target.S3Key))",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated code missing %q:\n%s", want, code)
		}
	}
	if _, err := format.Source([]byte("package out\n" + code)); err != nil {
		t.Fatalf("generated create function failed gofmt: %v\n%s", err, code)
	}

	// Optional pointer target emits both ClearIPV4 and dereferenced SetIPV4.
	optEntity := conf.NewEntity(
		taggedAcronymSource{},
		optionalAcronymTarget{},
		[]any{taggedAcronymUpdateOne{}},
		conf.WithCustomFieldConverter("ip_v4", ConvertPBIPV4),
	)
	optCode, err := GenBindFunc(taggedAcronymUpdateOne{}, optEntity, optEntity.CustomFieldConverters, true)
	if err != nil {
		t.Fatalf("GenBindFunc optional update failed: %v", err)
	}
	for _, want := range []string{
		"source.ClearIPV4()",
		"source.SetIPV4(bind.ConvertPBIPV4(*target.IpV4))",
		"source.SetHTTPURL(target.HttpUrl)",
		"source.SetS3Key(target.S3Key)",
	} {
		if !strings.Contains(optCode, want) {
			t.Fatalf("generated optional update code missing %q:\n%s", want, optCode)
		}
	}

	// Legacy strcase.ToSnake(field.Name) keys ("ipv4", "s3key") also resolve converters,
	// and untagged target fields still match via fallback.
	fallbackEntity := conf.NewEntity(
		taggedAcronymSource{},
		untaggedAcronymTarget{},
		[]any{taggedAcronymCreate{}},
		conf.WithCustomFieldConverter("ipv4", ConvertPBIPV4),
		conf.WithCustomFieldConverter("s3key", NormalizeS3Key),
	)
	fallbackCode, err := GenBindFunc(taggedAcronymCreate{}, fallbackEntity, fallbackEntity.CustomFieldConverters, true)
	if err != nil {
		t.Fatalf("GenBindFunc fallback failed: %v", err)
	}
	for _, want := range []string{
		"SetIPV4(bind.ConvertPBIPV4(target.IPV4))",
		"SetHTTPURL(target.HTTPURL)",
		"SetS3Key(bind.NormalizeS3Key(target.S3Key))",
	} {
		if !strings.Contains(fallbackCode, want) {
			t.Fatalf("generated fallback code missing %q:\n%s", want, fallbackCode)
		}
	}

	// IgnoreFields honors both canonical tag names and Go/snake field names.
	for _, ignoreList := range [][]string{
		{"ip_v4", "http_url", "s3_key"},
		{"IPV4", "HTTPURL", "S3Key"},
	} {
		ignoredEntity := conf.NewEntity(
			taggedAcronymSource{},
			taggedAcronymTarget{},
			[]any{taggedAcronymCreate{}},
			conf.WithIgnoreFields(ignoreList...),
		)
		ignoredCode, err := GenBindFunc(taggedAcronymCreate{}, ignoredEntity, nil, true)
		if err != nil {
			t.Fatalf("GenBindFunc with ignore %v failed: %v", ignoreList, err)
		}
		for _, forbidden := range []string{"SetIPV4", "SetHTTPURL", "SetS3Key"} {
			if strings.Contains(ignoredCode, forbidden) {
				t.Fatalf("ignore %v should omit %s, got:\n%s", ignoreList, forbidden, ignoredCode)
			}
		}
	}
}
