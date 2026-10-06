package entproto

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-viper/mapstructure/v2"

	"entgo.io/ent/entc/gen"
)

// EnumAnnotation is the annotation name under which [Enum] is stored in ent
// field annotations.
const (
	EnumAnnotation = "ProtoEnum"
)

var (
	// ErrEnumFieldsNotAnnotated is returned when an [Enum] annotation does not
	// list exactly as many values as the ent enum field defines.
	ErrEnumFieldsNotAnnotated = errors.New("entproto: all Enum options must be covered with an entproto.Enum annotation")
	normalizeEnumIdent        = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
)

// EnumOption configures the annotation returned by [Enum].
type EnumOption func(*enum)

// Enum maps each value of an ent enum field to a protobuf enum number. Use it
// together with [Field] on the same field. vals must contain every ent enum
// value and nothing else, and protobuf identifiers derived from the values must
// not collide after [NormalizeEnumIdentifier].
//
// Number 0 is special: if the ent field has a default, the default value must
// map to 0; otherwise no value may use 0, and entproto emits
// <FIELD>_UNSPECIFIED = 0 itself. The generated enum is nested in the message
// and named after the field in PascalCase; value names are
// <FIELD>_<VALUE> in upper snake case unless [OmitFieldPrefix] is set.
// Violations are reported when the schema is converted, not when Enum is
// called.
func Enum(vals map[string]int32, opts ...EnumOption) *enum {
	// apply options
	e := &enum{Options: vals}
	for _, op := range opts {
		op(e)
	}
	return e
}

// OmitFieldPrefix configures the Enum to omit the field name prefix from
// the enum labels on the generated protobuf message. Used for backwards
// compatibility with earlier versions of entproto where the field name
// wasn't prepended to the enum labels.
func OmitFieldPrefix() EnumOption {
	return func(e *enum) {
		e.OmitFieldPrefix = true
	}
}

type enum struct {
	Options         map[string]int32
	OmitFieldPrefix bool
}

func (*enum) Name() string {
	return EnumAnnotation
}

func (e *enum) findByNumber(n int32) string {
	for k, v := range e.Options {
		if v == n {
			return k
		}
	}
	return ""
}

func (e *enum) Verify(fld *gen.Field) error {
	// Verify that all fields on the Enum are in the annotation.
	if len(e.Options) != len(fld.Enums) {
		return ErrEnumFieldsNotAnnotated
	}
	pbIdentifiers := make(map[string]struct{}, len(fld.Enums))
	for _, opt := range fld.Enums {
		if _, ok := e.Options[opt.Value]; !ok {
			return fmt.Errorf("entproto: Enum option %s is not annotated with"+
				" a pbfield number using entproto.Enum", opt.Name)
		}
		pbIdent := NormalizeEnumIdentifier(opt.Value)
		if _, ok := pbIdentifiers[pbIdent]; ok {
			return fmt.Errorf("entproto: Enum option %q produces conflicting pbfield"+
				" name %q after normalization", opt.Name, pbIdent)
		}
		pbIdentifiers[pbIdent] = struct{}{}
	}

	// If default value is set on the pbfield, make sure it's option number is zero.
	if fld.Default {
		dv, ok := fld.DefaultValue().(string)
		if !ok {
			return fmt.Errorf("entproto: default value on Enum pbfield %s should be a string", fld.Name)
		}
		zeroField := e.findByNumber(0)
		if zeroField == "" {
			return fmt.Errorf("entproto: Enum pbfield %q has a default value but"+
				" entproto.Enum annotation doesn't contain an option with number 0", fld.Name)
		}
		if zeroField != dv {
			return fmt.Errorf(
				"entproto: default value for Enum pbfield %q is %q, but the proto annotation pbfield 0 is %q",
				fld.Name, dv, zeroField)
		}
	} else {
		// Make sure no one is using the zero option number.
		zeroField := e.findByNumber(0)
		if zeroField != "" {
			return fmt.Errorf("entproto: Enum pbfield %q has no default value but"+
				" entproto.Enum annotation contains an option with number 0", fld.Name)
		}
	}

	return nil
}

// EnumValueNumber returns the protobuf number that the [Enum] annotation on fld
// assigns to the ent enum value. ok is false when fld has no decodable Enum
// annotation or the annotation does not list value. It lets consumers resolve
// the real protobuf value descriptor (and therefore its generated Go constant
// name) instead of re-deriving it from naming conventions.
func EnumValueNumber(fld *gen.Field, value string) (int32, bool) {
	ann, err := extractEnumAnnotation(fld)
	if err != nil {
		return 0, false
	}
	n, ok := ann.Options[value]
	return n, ok
}

func extractEnumAnnotation(fld *gen.Field) (*enum, error) {
	annot, ok := fld.Annotations[EnumAnnotation]
	if !ok {
		return nil, fmt.Errorf("entproto: field %q does not have an entproto.Enum annotation", fld.Name)
	}

	var out enum
	err := mapstructure.Decode(annot, &out)
	if err != nil {
		return nil, fmt.Errorf("entproto: unable to decode entproto.Enum annotation for field %q: %w",
			fld.Name, err)
	}

	return &out, nil
}

// NormalizeEnumIdentifier upper-cases s and replaces every run of characters
// other than ASCII letters, digits, and underscore with a single underscore,
// for example "in-progress" -> "IN_PROGRESS". entproto applies it to ent enum
// values before building protobuf enum value names.
func NormalizeEnumIdentifier(s string) string {
	return strings.ToUpper(normalizeEnumIdent.ReplaceAllString(s, "_"))
}
