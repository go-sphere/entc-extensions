package entproto

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidAnnotation matches every [*InvalidAnnotationError] with
	// errors.Is.
	ErrInvalidAnnotation = errors.New("entproto: invalid annotation")
	// ErrFieldNumberOverflow matches every [*FieldNumberOverflowError] with
	// errors.Is.
	ErrFieldNumberOverflow = errors.New("entproto: field number overflow")
	// ErrDuplicateFieldNumber matches every [*DuplicateFieldNumberError] with
	// errors.Is.
	ErrDuplicateFieldNumber = errors.New("entproto: duplicate field number")
)

// InvalidAnnotationError reports an entproto annotation that cannot be
// decoded, as returned by [FixGraph]. It matches [ErrInvalidAnnotation] with
// errors.Is and unwraps to Cause.
type InvalidAnnotationError struct {
	Schema     string // ent schema name
	Field      string // field name; empty for schema-level or edge annotations
	Edge       string // edge name; empty unless the annotation is on an edge
	Annotation string // annotation name, such as [FieldAnnotation]
	Cause      error  // underlying decode error
}

func (e *InvalidAnnotationError) Error() string {
	loc := e.Schema
	if e.Field != "" {
		loc += "." + e.Field
	}
	if e.Edge != "" {
		loc += "." + e.Edge
	}
	if e.Annotation != "" {
		return fmt.Sprintf("entproto: invalid annotation %q on %s: %v", e.Annotation, loc, e.Cause)
	}
	return fmt.Sprintf("entproto: invalid annotation on %s: %v", loc, e.Cause)
}

func (e *InvalidAnnotationError) Unwrap() error {
	return e.Cause
}

func (*InvalidAnnotationError) Is(target error) bool {
	return target == ErrInvalidAnnotation
}

// FieldNumberOverflowError reports a field or edge whose [Field] number does
// not fit in int32; Field holds the field or edge name. It is the conversion
// error of the schema (see [Adapter.GetFileDescriptor]) and matches
// [ErrFieldNumberOverflow] with errors.Is.
type FieldNumberOverflowError struct {
	Schema string
	Field  string
	Number int
}

func (e *FieldNumberOverflowError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("entproto: field number overflow on %s.%s: %d", e.Schema, e.Field, e.Number)
	}
	return fmt.Sprintf("entproto: field number overflow on %s: %d", e.Schema, e.Number)
}

func (*FieldNumberOverflowError) Is(target error) bool {
	return target == ErrFieldNumberOverflow
}

// DuplicateFieldNumberError reports two fields or edges of one message that use
// the same protobuf field number. It is the conversion error of that schema
// (see [Adapter.GetFileDescriptor]) and matches [ErrDuplicateFieldNumber] with
// errors.Is.
type DuplicateFieldNumberError struct {
	Message string
	Number  int32
}

func (e *DuplicateFieldNumberError) Error() string {
	return fmt.Sprintf("entproto: duplicate field number %d in message %q", e.Number, e.Message)
}

func (*DuplicateFieldNumberError) Is(target error) bool {
	return target == ErrDuplicateFieldNumber
}

// DanglingReferenceError reports a schema that references another schema (via
// an edge or message-typed field) whose descriptor failed to build. It is
// returned by [LoadAdapter] and unwraps to the referenced schema's error.
type DanglingReferenceError struct {
	Schema    string
	Field     string
	RefSchema string
	Cause     error
}

func (e *DanglingReferenceError) Error() string {
	return fmt.Sprintf(
		"entproto: %s.%s references schema %q which failed to generate: %v",
		e.Schema, e.Field, e.RefSchema, e.Cause,
	)
}

func (e *DanglingReferenceError) Unwrap() error {
	return e.Cause
}
