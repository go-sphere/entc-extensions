package entproto

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidAnnotation indicates an annotation cannot be decoded or validated.
	ErrInvalidAnnotation = errors.New("entproto: invalid annotation")
	// ErrFieldNumberOverflow indicates a protobuf field number exceeded the supported range.
	ErrFieldNumberOverflow = errors.New("entproto: field number overflow")
	// ErrDuplicateFieldNumber indicates a message has duplicate field numbers.
	ErrDuplicateFieldNumber = errors.New("entproto: duplicate field number")
)

// InvalidAnnotationError describes an invalid schema/field annotation.
type InvalidAnnotationError struct {
	Schema     string
	Field      string
	Edge       string
	Annotation string
	Cause      error
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

// FieldNumberOverflowError describes a field number overflow.
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

// DuplicateFieldNumberError describes duplicate field numbers in a message.
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
