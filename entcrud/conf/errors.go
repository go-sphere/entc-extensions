package conf

import (
	"fmt"
	"strings"
)

// TypeMismatchError describes a non-convertible source/target field pair.
type TypeMismatchError struct {
	Entity     string
	Field      string
	SourceType string
	TargetType string
	Suggestion string
}

func (e TypeMismatchError) Error() string {
	if e.Suggestion == "" {
		return fmt.Sprintf(
			"entcrud: field type mismatch on %s.%s (%s <- %s)",
			e.Entity, e.Field, e.SourceType, e.TargetType,
		)
	}
	return fmt.Sprintf(
		"entcrud: field type mismatch on %s.%s (%s <- %s): %s",
		e.Entity, e.Field, e.SourceType, e.TargetType, e.Suggestion,
	)
}

// TypeMismatchListError aggregates multiple type mismatches in one generation run.
type TypeMismatchListError struct {
	Items []TypeMismatchError
}

func (e *TypeMismatchListError) Error() string {
	if e == nil || len(e.Items) == 0 {
		return "entcrud: type mismatch"
	}
	parts := make([]string, 0, len(e.Items))
	for _, item := range e.Items {
		parts = append(parts, item.Error())
	}
	return strings.Join(parts, "; ")
}
