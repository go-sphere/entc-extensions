package conf

import (
	"fmt"
	"strings"
)

// TypeMismatchError describes one protobuf field that cannot be bound to its
// ent setter, either because the types are neither assignable nor convertible
// or because the registered custom converter does not fit.
type TypeMismatchError struct {
	Entity     string // ent entity type name, such as "User"
	Field      string // ent struct field name, such as "Birthday"
	SourceType string // Go type of the ent setter argument
	TargetType string // Go type of the protobuf struct field
	Suggestion string // how to fix it; may be empty
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

// TypeMismatchListError is returned by
// [github.com/go-sphere/entc-extensions/entcrud.BindFiles] under
// [FilesConf.StrictTypeCheck]. Items lists every mismatched field of the first
// action that had any; fix them and rerun to see later ones. Use errors.As to
// obtain it.
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
